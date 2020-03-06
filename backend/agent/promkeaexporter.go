package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	storkutil "isc.org/stork/util"
)

// Settings for Prometheus Kea Exporter
type PromKeaExporterSettings struct {
	Host     string `long:"prometheus-kea-exporter-host" description:"the IP to listen on" default:"0.0.0.0" env:"STORK_AGENT_PROMETHEUS_KEA_EXPORTER_ADDRESS"`
	Port     int    `long:"prometheus-kea-exporter-port" description:"the port to listen on for connections" default:"9547" env:"STORK_AGENT_PROMETHEUS_KEA_EXPORTER_PORT"`
	Interval int    `long:"prometheus-kea-exporter-interval" description:"interval of collecting Kea stats in seconds" default:"10" env:"STORK_AGENT_PROMETHEUS_KEA_EXPORTER_INTERVAL"`
}

// Stats descriptor that holds reference to prometheus stats
// and its 'operation' label.
type statDescr struct {
	Stat      *prometheus.GaugeVec
	Operation string
}

// Main structure for Prometheus Kea Exporter. It holds its settings,
// references to app monitor, CA client, HTTP server, and main loop
// controlling elements like ticker, and mappings between kea stats
// names to prometheus stats.
type PromKeaExporter struct {
	Settings PromKeaExporterSettings

	AppMonitor AppMonitor
	CAClient   *CAClient
	HTTPServer *http.Server

	Ticker        *time.Ticker
	DoneCollector chan bool
	Wg            *sync.WaitGroup

	PktStatsMap  map[string]statDescr
	Adr4StatsMap map[string]*prometheus.GaugeVec
	Adr6StatsMap map[string]*prometheus.GaugeVec
}

// Create new Prometheus Kea Exporter.
func NewPromKeaExporter(appMonitor AppMonitor) *PromKeaExporter {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Handler: mux,
	}
	pke := &PromKeaExporter{
		AppMonitor:    appMonitor,
		CAClient:      NewCAClient(),
		HTTPServer:    srv,
		DoneCollector: make(chan bool),
		Wg:            &sync.WaitGroup{},
	}

	// packets dhcp4
	packets4SentTotal := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp4",
		Name:      "packets_sent_total",
		Help:      "Packets sent",
	}, []string{"operation"})
	packets4ReceivedTotal := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp4",
		Name:      "packets_received_total",
		Help:      "Packets received",
	}, []string{"operation"})

	// packets dhcp6
	packets6SentTotal := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "packets_sent_total",
		Help:      "Packets sent",
	}, []string{"operation"})
	packets6ReceivedTotal := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "packets_received_total",
		Help:      "Packets received",
	}, []string{"operation"})
	packets4o6SentTotal := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "packets_sent_dhcp4_total",
		Help:      "DHCPv4-over-DHCPv6 Packets received",
	}, []string{"operation"})
	packets4o6ReceivedTotal := promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "packets_received_dhcp4_total",
		Help:      "DHCPv4-over-DHCPv6 Packets received",
	}, []string{"operation"})

	pktStatsMap := make(map[string]statDescr)

	// packets4ReceivedTotal
	pktStatsMap["pkt4-nak-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "nak"}
	pktStatsMap["pkt4-ack-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "ack"}
	pktStatsMap["pkt4-decline-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "decline"}
	pktStatsMap["pkt4-discover-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "discover"}
	pktStatsMap["pkt4-inform-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "inform"}
	pktStatsMap["pkt4-offer-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "offer"}
	pktStatsMap["pkt4-receive-drop"] = statDescr{Stat: packets4ReceivedTotal, Operation: "drop"}
	pktStatsMap["pkt4-parse-failed"] = statDescr{Stat: packets4ReceivedTotal, Operation: "parse-failed"}
	pktStatsMap["pkt4-release-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "release"}
	pktStatsMap["pkt4-request-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "request"}
	pktStatsMap["pkt4-unknown-received"] = statDescr{Stat: packets4ReceivedTotal, Operation: "unknown"}

	// packets4SentTotal
	pktStatsMap["pkt4-offer-sent"] = statDescr{Stat: packets4SentTotal, Operation: "offer"}
	pktStatsMap["pkt4-nak-sent"] = statDescr{Stat: packets4SentTotal, Operation: "nak"}
	pktStatsMap["pkt4-ack-sent"] = statDescr{Stat: packets4SentTotal, Operation: "ack"}

	// packets6ReceivedTotal
	pktStatsMap["pkt6-receive-drop"] = statDescr{Stat: packets6ReceivedTotal, Operation: "drop"}
	pktStatsMap["pkt6-parse-failed"] = statDescr{Stat: packets6ReceivedTotal, Operation: "parse-failed"}
	pktStatsMap["pkt6-solicit-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "solicit"}
	pktStatsMap["pkt6-advertise-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "advertise"}
	pktStatsMap["pkt6-request-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "request"}
	pktStatsMap["pkt6-reply-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "reply"}
	pktStatsMap["pkt6-renew-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "renew"}
	pktStatsMap["pkt6-rebind-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "rebind"}
	pktStatsMap["pkt6-release-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "release"}
	pktStatsMap["pkt6-decline-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "decline"}
	pktStatsMap["pkt6-infrequest-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "infrequest"}
	pktStatsMap["pkt6-unknown-received"] = statDescr{Stat: packets6ReceivedTotal, Operation: "unknown"}

	// packets6SentTotal
	pktStatsMap["pkt6-advertise-sent"] = statDescr{Stat: packets6SentTotal, Operation: "advertise"}
	pktStatsMap["pkt6-reply-sent"] = statDescr{Stat: packets6SentTotal, Operation: "reply"}

	// packets4o6SentTotal & packets4o6ReceivedTotal
	pktStatsMap["pkt6-dhcpv4-response-sent"] = statDescr{Stat: packets4o6SentTotal, Operation: "response"}
	pktStatsMap["pkt6-dhcpv4-query-received"] = statDescr{Stat: packets4o6ReceivedTotal, Operation: "query"}
	pktStatsMap["pkt6-dhcpv4-response-received"] = statDescr{Stat: packets4o6ReceivedTotal, Operation: "response"}

	// addresses dhcp4
	adr4StatsMap := make(map[string]*prometheus.GaugeVec)
	adr4StatsMap["assigned-addresses"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp4",
		Name:      "addresses_assigned_total",
		Help:      "Assigned addresses",
	}, []string{"subnet"})
	adr4StatsMap["declined-addresses"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp4",
		Name:      "addresses_declined_total",
		Help:      "Declined counts",
	}, []string{"subnet"})
	adr4StatsMap["reclaimed-declined-addresses"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp4",
		Name:      "addresses_declined_reclaimed_total",
		Help:      "Declined addresses that were reclaimed",
	}, []string{"subnet"})
	adr4StatsMap["reclaimed-leases"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp4",
		Name:      "addresses_reclaimed_total",
		Help:      "Expired addresses that were reclaimed",
	}, []string{"subnet"})
	adr4StatsMap["total-addresses"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp4",
		Name:      "addresses_total",
		Help:      "Size of subnet address pool",
	}, []string{"subnet"})

	// addresses dhcp6
	adr6StatsMap := make(map[string]*prometheus.GaugeVec)
	adr6StatsMap["total-nas"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "na_total",
		Help:      "'Size of non-temporary address pool",
	}, []string{"subnet"})
	adr6StatsMap["assigned-nas"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "na_assigned_total",
		Help:      "Assigned non-temporary addresses (IA_NA)",
	}, []string{"subnet"})
	adr6StatsMap["total-pds"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "pd_total",
		Help:      "Size of prefix delegation pool",
	}, []string{"subnet"})
	adr6StatsMap["assigned-pds"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "pd_assigned_total",
		Help:      "Assigned prefix delegations (IA_PD)",
	}, []string{"subnet"})
	adr6StatsMap["reclaimed-leases"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "addresses_reclaimed_total",
		Help:      "Expired addresses that were reclaimed",
	}, []string{"subnet"})
	adr6StatsMap["declined-addresses"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "addresses_declined_total",
		Help:      "Declined counts",
	}, []string{"subnet"})
	adr6StatsMap["reclaimed-declined-addresses"] = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: AppTypeKea,
		Subsystem: "dhcp6",
		Name:      "addresses_declined_reclaimed_total",
		Help:      "Declined addresses that were reclaimed",
	}, []string{"subnet"})

	pke.PktStatsMap = pktStatsMap
	pke.Adr4StatsMap = adr4StatsMap
	pke.Adr6StatsMap = adr6StatsMap

	return pke
}

// Start goroutine with main loop for collecting stats
// and http server for exposing them to Prometheus.
func (pke *PromKeaExporter) Start() {
	// set address for listening from settings
	addrPort := fmt.Sprintf("%s:%d", pke.Settings.Host, pke.Settings.Port)
	pke.HTTPServer.Addr = addrPort

	log.Printf("Prometheus Kea Exporter listening on %s, stats pulling interval: %d seconds", addrPort, pke.Settings.Interval)

	// start http server for metrics
	go func() {
		err := pke.HTTPServer.ListenAndServe()
		if err != nil {
			log.Errorf("problem with serving Prometheus Kea Exporter: %s", err.Error())
		}
	}()

	// set ticker time for collecting loop from settings
	pke.Ticker = time.NewTicker(time.Duration(pke.Settings.Interval) * time.Second)

	// start collecting loop as goroutine and increment WaitGroup (which is used later
	// for stopping this goroutine)
	pke.Wg.Add(1)
	go pke.statsCollectorLoop()
}

// Shutdown exporter goroutines and unregister prometheus stats.
func (pke *PromKeaExporter) Shutdown() {
	log.Printf("Stopping Prometheus Kea Exporter")

	// stop http server
	if pke.HTTPServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pke.HTTPServer.SetKeepAlivesEnabled(false)
		if err := pke.HTTPServer.Shutdown(ctx); err != nil {
			log.Warnf("Could not gracefully shutdown the kea exporter: %v\n", err)
		}
	}

	// stop stats collector
	if pke.Ticker != nil {
		pke.Ticker.Stop()
		pke.DoneCollector <- true
		pke.Wg.Wait()
	}

	// unregister kea counters from prometheus framework
	prometheus.Unregister(pke.PktStatsMap["pkt4-nak-received"].Stat)
	prometheus.Unregister(pke.PktStatsMap["pkt4-offer-sent"].Stat)
	prometheus.Unregister(pke.PktStatsMap["pkt6-receive-drop"].Stat)
	prometheus.Unregister(pke.PktStatsMap["pkt6-advertise-sent"].Stat)
	prometheus.Unregister(pke.PktStatsMap["pkt6-dhcpv4-response-sent"].Stat)
	prometheus.Unregister(pke.PktStatsMap["pkt6-dhcpv4-query-received"].Stat)
	for _, stat := range pke.Adr4StatsMap {
		prometheus.Unregister(stat)
	}
	for _, stat := range pke.Adr6StatsMap {
		prometheus.Unregister(stat)
	}

	log.Printf("Stopped Prometheus Kea Exporter")
}

// Main loop for collecting stats periodically.
func (pke *PromKeaExporter) statsCollectorLoop() {
	defer pke.Wg.Done()
	for {
		select {
		// every N seconds do stats collection from all kea and its active daemons
		case <-pke.Ticker.C:
			err := pke.collectStats()
			if err != nil {
				log.Errorf("some errors were encountered while collecting stats from kea: %+v", err)
			}
		// wait for done signal from shutdown function
		case <-pke.DoneCollector:
			return
		}
	}
}

// setDaemonStats stores the stat values from a daemon in the proper prometheus object.
func (pke *PromKeaExporter) setDaemonStats(daemonIdx int, rspIfc interface{}, ignoredStats map[string]bool) error {
	rsp, ok := rspIfc.(map[string]interface{})
	if !ok {
		return errors.Errorf("problem with casting rspIfc: %+v", rspIfc)
	}

	resultIfc, ok := rsp["result"]
	if !ok {
		return errors.Errorf("no 'result' in response: %+v", rsp)
	}

	result, ok := resultIfc.(float64)
	if !ok {
		return errors.Errorf("problem with casting resultIfc: %+v", resultIfc)
	}
	if result != 0 {
		textIfc, ok := rsp["text"]
		if ok {
			text, ok := textIfc.(string)
			if ok && (!strings.Contains(text, "server is likely to be offline") || !strings.Contains(text, "forwarding socket is not configured for the server type")) {
				return errors.Errorf("response result from Kea != 0: %d, text: %s", int(result), text)
			}
		}
		return errors.Errorf("response result from Kea != 0: %d", int(result))
	}

	argsIfc, ok := rsp["arguments"]
	if !ok {
		return errors.Errorf("no 'arguments' in response: %+v", rsp)
	}

	args := argsIfc.(map[string]interface{})
	if !ok {
		return errors.Errorf("problem with casting argsIfc: %+v", argsIfc)
	}

	for statName, statValueList1Ifc := range args {
		// skip ignored stats
		if ignoredStats[statName] {
			continue
		}

		// get stat value from nested lists (eg. [[val, timestamp]])
		statValueList1, ok := statValueList1Ifc.([]interface{})
		if !ok {
			log.Errorf("problem with casting statValueList1Ifc: %+v", statValueList1Ifc)
			continue
		}
		if len(statValueList1) == 0 {
			log.Errorf("empty list of stat values")
			continue
		}
		statValueList2, ok := statValueList1[0].([]interface{})
		if !ok {
			log.Errorf("problem with casting statValueList1[0]: %+v", statValueList1[0])
			continue
		}
		if len(statValueList2) == 0 {
			log.Errorf("empty list of stat values")
			continue
		}
		statValue, ok := statValueList2[0].(float64)
		if !ok {
			log.Errorf("problem with casting statValueList2[0]: %+v", statValueList2[0])
			continue
		}

		// store stat value in proper prometheus object
		if strings.HasPrefix(statName, "pkt") {
			// if this is pkt stat
			statDescr := pke.PktStatsMap[statName]
			statDescr.Stat.With(prometheus.Labels{"operation": statDescr.Operation}).Set(statValue)
		} else if strings.HasPrefix(statName, "subnet[") {
			// if this is address per subnet stat
			re := regexp.MustCompile(`subnet\[(\d+)\]\.(.+)`)
			matches := re.FindStringSubmatch(statName)
			subnetID := matches[1]
			name := matches[2]

			var stat *prometheus.GaugeVec
			// daemon 0 is dhcp4, 1 is dhcp6
			if daemonIdx == 0 {
				stat = pke.Adr4StatsMap[name]
			} else {
				stat = pke.Adr6StatsMap[name]
			}
			stat.With(prometheus.Labels{"subnet": subnetID}).Set(statValue)
		}
	}

	return nil
}

// Collect stats from all Kea apps.
func (pke *PromKeaExporter) collectStats() error {
	var lastErr error
	// these stats are ignored as they are estimated by summing sub-stats (like ack, nak, etc)
	ignoredStats := map[string]bool{
		"pkt4-received": true,
		"pkt4-sent":     true,
		"pkt6-received": true,
		"pkt6-sent":     true,
	}

	// Request to kea dhcp daemons for getting all stats. Both v4 and v6 is queried because
	// here we do not have knowledge which are active.
	request := `{
             "command":"statistic-get-all",
             "service":["dhcp4", "dhcp6"],
             "arguments": {}
        }`

	// go through all kea apps discovered by monitor and query them for stats
	apps := pke.AppMonitor.GetApps()
	for _, app := range apps {
		// ignore non-kea apps
		if app.Type != AppTypeKea {
			continue
		}
		log.Printf("APP %+v", app)

		// get stats from kea
		ctrl, err := GetAccessPoint(app, AccessPointControl)
		if err != nil {
			lastErr = err
			log.Errorf("problem with getting stats from kea: %+v", err)
			continue
		}
		caURL := storkutil.HostWithPortURL(ctrl.Address, ctrl.Port)
		httpRsp, err := pke.CAClient.Call(caURL, bytes.NewBuffer([]byte(request)))
		if err != nil {
			lastErr = err
			log.Errorf("problem with getting stats from kea: %+v", err)
			continue
		}
		body, err := ioutil.ReadAll(httpRsp.Body)
		httpRsp.Body.Close()
		if err != nil {
			lastErr = err
			log.Errorf("problem with reading stats response from kea: %+v", err)
			continue
		}
		response := string(body)

		// parse response
		var rspsIfc interface{}
		err = json.Unmarshal([]byte(response), &rspsIfc)
		if err != nil {
			lastErr = err
			log.Errorf("failed to parse responses from Kea: %s", err)
			continue
		}
		rspList, ok := rspsIfc.([]interface{})
		if !ok {
			lastErr = errors.Errorf("problem with casting rspsIfc: %+v", rspsIfc)
			log.Errorf("%+v", lastErr)
			continue
		}

		// go though list of responses from daemons (it can have none or some responses from dhcp4/dhcp6)
		for daemonIdx, rspIfc := range rspList {
			err = pke.setDaemonStats(daemonIdx, rspIfc, ignoredStats)
			if err != nil {
				log.Errorf("cannot get stat from daemon: %+v", err)
			}
		}
	}
	return lastErr
}
