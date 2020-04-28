package restservice

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	dbmodel "isc.org/stork/server/database/model"
	dbtest "isc.org/stork/server/database/test"
	"isc.org/stork/server/gen/restapi/operations/search"
	storktest "isc.org/stork/server/test"
)

// Check searching via rest api functions.
func TestSearchRecords(t *testing.T) {
	db, dbSettings, teardown := dbtest.SetupDatabaseTestCase(t)
	defer teardown()

	settings := RestAPISettings{}
	fa := storktest.NewFakeAgents(nil, nil)
	rapi, err := NewRestAPI(&settings, dbSettings, db, fa)
	require.NoError(t, err)
	ctx := context.Background()

	// search with empty text
	params := search.SearchRecordsParams{}
	rsp := rapi.SearchRecords(ctx, params)
	require.IsType(t, &search.SearchRecordsOK{}, rsp)
	okRsp := rsp.(*search.SearchRecordsOK)
	require.Len(t, okRsp.Payload.Apps.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Apps.Total)
	require.Len(t, okRsp.Payload.Groups.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Groups.Total)
	require.Len(t, okRsp.Payload.Hosts.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Hosts.Total)
	require.Len(t, okRsp.Payload.Machines.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Machines.Total)
	require.Len(t, okRsp.Payload.SharedNetworks.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.SharedNetworks.Total)
	require.Len(t, okRsp.Payload.Subnets.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Subnets.Total)
	require.Len(t, okRsp.Payload.Users.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Users.Total)

	// add machine
	m := &dbmodel.Machine{
		Address:   "localhost",
		AgentPort: 8080,
	}
	err = dbmodel.AddMachine(db, m)
	require.NoError(t, err)

	// add app kea with dhcp4 to machine
	var accessPoints []*dbmodel.AccessPoint
	accessPoints = dbmodel.AppendAccessPoint(accessPoints, dbmodel.AccessPointControl, "", "", 1114)

	a4 := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: accessPoints,
		Daemons: []*dbmodel.Daemon{
			{
				KeaDaemon: &dbmodel.KeaDaemon{
					Config: dbmodel.NewKeaConfig(&map[string]interface{}{
						"Dhcp4": &map[string]interface{}{
							"subnet4": []map[string]interface{}{{
								"id":     1,
								"subnet": "192.168.0.0/24",
								"pools": []map[string]interface{}{{
									"pool": "192.168.0.1-192.168.0.100",
								}, {
									"pool": "192.168.0.150-192.168.0.200",
								}},
							}},
						},
					}),
				},
			},
		},
	}
	err = dbmodel.AddApp(db, a4)
	require.NoError(t, err)

	appSubnets := []dbmodel.Subnet{
		{
			Prefix: "192.168.0.0/24",
			AddressPools: []dbmodel.AddressPool{
				{
					LowerBound: "192.168.0.1",
					UpperBound: "192.168.0.100",
				},
				{
					LowerBound: "192.168.0.150",
					UpperBound: "192.168.0.200",
				},
			},
		},
	}

	err = dbmodel.CommitNetworksIntoDB(db, []dbmodel.SharedNetwork{}, appSubnets, a4, 1)
	require.NoError(t, err)

	// add app kea with dhcp6 to machine
	accessPoints = []*dbmodel.AccessPoint{}
	accessPoints = dbmodel.AppendAccessPoint(accessPoints, dbmodel.AccessPointControl, "", "", 1116)

	a6 := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: accessPoints,
		Daemons: []*dbmodel.Daemon{
			{
				KeaDaemon: &dbmodel.KeaDaemon{
					Config: dbmodel.NewKeaConfig(&map[string]interface{}{
						"Dhcp6": &map[string]interface{}{
							"subnet6": []map[string]interface{}{{
								"id":     2,
								"subnet": "2001:db8:1::/64",
								"pools":  []map[string]interface{}{},
							}},
						},
					}),
				},
			},
		},
	}
	err = dbmodel.AddApp(db, a6)
	require.NoError(t, err)

	appSubnets = []dbmodel.Subnet{
		{
			Prefix: "2001:db8:1::/64",
		},
	}
	err = dbmodel.CommitNetworksIntoDB(db, []dbmodel.SharedNetwork{}, appSubnets, a6, 1)
	require.NoError(t, err)

	// add app kea with dhcp4 and dhcp6 to machine
	accessPoints = []*dbmodel.AccessPoint{}
	accessPoints = dbmodel.AppendAccessPoint(accessPoints, dbmodel.AccessPointControl, "", "", 1146)

	a46 := &dbmodel.App{
		ID:           0,
		MachineID:    m.ID,
		Type:         dbmodel.AppTypeKea,
		Active:       true,
		AccessPoints: accessPoints,
		Daemons: []*dbmodel.Daemon{
			{
				KeaDaemon: &dbmodel.KeaDaemon{
					Config: dbmodel.NewKeaConfig(&map[string]interface{}{
						"Dhcp4": &map[string]interface{}{
							"subnet4": []map[string]interface{}{{
								"id":     3,
								"subnet": "192.118.0.0/24",
								"pools": []map[string]interface{}{{
									"pool": "192.118.0.1-192.118.0.200",
								}},
							}},
						},
					}),
				},
			},
			{
				KeaDaemon: &dbmodel.KeaDaemon{
					Config: dbmodel.NewKeaConfig(&map[string]interface{}{
						"Dhcp6": &map[string]interface{}{
							"subnet6": []map[string]interface{}{{
								"id":     4,
								"subnet": "3001:db8:1::/64",
								"pools": []map[string]interface{}{{
									"pool": "3001:db8:1::/80",
								}},
							}},
							"shared-networks": []map[string]interface{}{{
								"name": "fox",
								"subnet6": []map[string]interface{}{{
									"id":     21,
									"subnet": "5001:db8:1::/64",
								}},
							}},
						},
					}),
				},
			},
		},
	}
	err = dbmodel.AddApp(db, a46)
	require.NoError(t, err)

	appNetworks := []dbmodel.SharedNetwork{
		{
			Name:   "fox",
			Family: 6,
			Subnets: []dbmodel.Subnet{
				{
					Prefix: "5001:db8:1::/64",
				},
			},
		},
	}

	appSubnets = []dbmodel.Subnet{
		{
			Prefix: "192.118.0.0/24",
			AddressPools: []dbmodel.AddressPool{
				{
					LowerBound: "192.118.0.1",
					UpperBound: "192.118.0.200",
				},
			},
		},
		{
			Prefix: "3001:db8:1::/64",
			AddressPools: []dbmodel.AddressPool{
				{
					LowerBound: "3001:db8:1::",
					UpperBound: "3001:db8:1:0:ffff::ffff",
				},
			},
		},
	}
	err = dbmodel.CommitNetworksIntoDB(db, appNetworks, appSubnets, a46, 1)
	require.NoError(t, err)

	// search for 'fox' - shared network and subnet are expected
	text := "fox"
	params = search.SearchRecordsParams{
		Text: &text,
	}
	rsp = rapi.SearchRecords(ctx, params)
	require.IsType(t, &search.SearchRecordsOK{}, rsp)
	okRsp = rsp.(*search.SearchRecordsOK)
	require.Len(t, okRsp.Payload.Apps.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Apps.Total)
	require.Len(t, okRsp.Payload.Groups.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Groups.Total)
	require.Len(t, okRsp.Payload.Hosts.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Hosts.Total)
	require.Len(t, okRsp.Payload.Machines.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Machines.Total)
	require.Len(t, okRsp.Payload.SharedNetworks.Items, 1)
	require.EqualValues(t, 1, okRsp.Payload.SharedNetworks.Total)
	require.Len(t, okRsp.Payload.Subnets.Items, 1)
	require.EqualValues(t, 1, okRsp.Payload.Subnets.Total)
	require.Len(t, okRsp.Payload.Users.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Users.Total)

	// search for '192.118.0.0/24' - subnet is expected
	text = "192.118.0.0/24"
	params = search.SearchRecordsParams{
		Text: &text,
	}
	rsp = rapi.SearchRecords(ctx, params)
	require.IsType(t, &search.SearchRecordsOK{}, rsp)
	okRsp = rsp.(*search.SearchRecordsOK)
	require.Len(t, okRsp.Payload.Apps.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Apps.Total)
	require.Len(t, okRsp.Payload.Groups.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Groups.Total)
	require.Len(t, okRsp.Payload.Hosts.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Hosts.Total)
	require.Len(t, okRsp.Payload.Machines.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Machines.Total)
	require.Len(t, okRsp.Payload.SharedNetworks.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.SharedNetworks.Total)
	require.Len(t, okRsp.Payload.Subnets.Items, 1)
	require.EqualValues(t, 1, okRsp.Payload.Subnets.Total)
	require.Len(t, okRsp.Payload.Users.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Users.Total)

	// search for 'super' - group is expected
	text = "super"
	params = search.SearchRecordsParams{
		Text: &text,
	}
	rsp = rapi.SearchRecords(ctx, params)
	require.IsType(t, &search.SearchRecordsOK{}, rsp)
	okRsp = rsp.(*search.SearchRecordsOK)
	require.Len(t, okRsp.Payload.Apps.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Apps.Total)
	require.Len(t, okRsp.Payload.Groups.Items, 1)
	require.EqualValues(t, 1, okRsp.Payload.Groups.Total)
	require.Len(t, okRsp.Payload.Hosts.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Hosts.Total)
	require.Len(t, okRsp.Payload.Machines.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Machines.Total)
	require.Len(t, okRsp.Payload.SharedNetworks.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.SharedNetworks.Total)
	require.Len(t, okRsp.Payload.Subnets.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Subnets.Total)
	require.Len(t, okRsp.Payload.Users.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Users.Total)

	// search for 'admin' - user and group are expected
	text = "admin"
	params = search.SearchRecordsParams{
		Text: &text,
	}
	rsp = rapi.SearchRecords(ctx, params)
	require.IsType(t, &search.SearchRecordsOK{}, rsp)
	okRsp = rsp.(*search.SearchRecordsOK)
	require.Len(t, okRsp.Payload.Apps.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Apps.Total)
	require.Len(t, okRsp.Payload.Groups.Items, 2)
	require.EqualValues(t, 2, okRsp.Payload.Groups.Total)
	require.Len(t, okRsp.Payload.Hosts.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Hosts.Total)
	require.Len(t, okRsp.Payload.Machines.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Machines.Total)
	require.Len(t, okRsp.Payload.SharedNetworks.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.SharedNetworks.Total)
	require.Len(t, okRsp.Payload.Subnets.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Subnets.Total)
	require.Len(t, okRsp.Payload.Users.Items, 1)
	require.EqualValues(t, 1, okRsp.Payload.Users.Total)

	// search for 'localhost' - machine is expected
	text = "localhost"
	params = search.SearchRecordsParams{
		Text: &text,
	}
	rsp = rapi.SearchRecords(ctx, params)
	require.IsType(t, &search.SearchRecordsOK{}, rsp)
	okRsp = rsp.(*search.SearchRecordsOK)
	require.Len(t, okRsp.Payload.Apps.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Apps.Total)
	require.Len(t, okRsp.Payload.Groups.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Groups.Total)
	require.Len(t, okRsp.Payload.Hosts.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Hosts.Total)
	require.Len(t, okRsp.Payload.Machines.Items, 1)
	require.EqualValues(t, 1, okRsp.Payload.Machines.Total)
	require.Len(t, okRsp.Payload.SharedNetworks.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.SharedNetworks.Total)
	require.Len(t, okRsp.Payload.Subnets.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Subnets.Total)
	require.Len(t, okRsp.Payload.Users.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Users.Total)

	// search for 'kea' - app is expected
	text = "kea"
	params = search.SearchRecordsParams{
		Text: &text,
	}
	rsp = rapi.SearchRecords(ctx, params)
	require.IsType(t, &search.SearchRecordsOK{}, rsp)
	okRsp = rsp.(*search.SearchRecordsOK)
	require.Len(t, okRsp.Payload.Apps.Items, 3)
	require.EqualValues(t, 3, okRsp.Payload.Apps.Total)
	require.Len(t, okRsp.Payload.Groups.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Groups.Total)
	require.Len(t, okRsp.Payload.Hosts.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Hosts.Total)
	require.Len(t, okRsp.Payload.Machines.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Machines.Total)
	require.Len(t, okRsp.Payload.SharedNetworks.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.SharedNetworks.Total)
	require.Len(t, okRsp.Payload.Subnets.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Subnets.Total)
	require.Len(t, okRsp.Payload.Users.Items, 0)
	require.EqualValues(t, 0, okRsp.Payload.Users.Total)
}
