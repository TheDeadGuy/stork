package restservice

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-openapi/runtime/middleware"
	log "github.com/sirupsen/logrus"

	dbmodel "isc.org/stork/server/database/model"
	"isc.org/stork/server/gen/models"
	"isc.org/stork/server/gen/restapi/operations/services"
)

// Get tail of the specified log file.
func (r *RestAPI) GetLogTail(ctx context.Context, params services.GetLogTailParams) middleware.Responder {
	// We have ID of the log file to display. We need to get the details
	// of the file from the database.
	dbLogTarget, err := dbmodel.GetLogTargetByID(r.Db, params.ID)
	if err != nil {
		msg := fmt.Sprintf("cannot get information about the log file with id %d from the database", params.ID)
		log.Error(msg)
		rsp := services.NewGetLogTailDefault(http.StatusInternalServerError).WithPayload(&models.APIError{
			Message: &msg,
		})
		return rsp
	}

	// Handle the case when referencing the non-existing file.
	if dbLogTarget == nil {
		msg := fmt.Sprintf("log file with id %d does not exist", params.ID)
		log.Warn(msg)
		rsp := services.NewGetLogTailDefault(http.StatusNotFound).WithPayload(&models.APIError{
			Message: &msg,
		})
		return rsp
	}

	// Currently we only support viewing log files.
	if dbLogTarget.Output == "stdout" || dbLogTarget.Output == "stderr" ||
		strings.HasPrefix(dbLogTarget.Output, "syslog") {
		msg := fmt.Sprintf("viewing log from %s is not supported", dbLogTarget.Output)
		log.Warn(msg)
		rsp := services.NewGetLogTailDefault(http.StatusBadRequest).WithPayload(&models.APIError{
			Message: &msg,
		})
		return rsp
	}

	// Send the request to the agent to tail the file.
	contents, err := r.Agents.TailTextFile(ctx, dbLogTarget.Daemon.App.Machine.Address,
		dbLogTarget.Daemon.App.Machine.AgentPort, dbLogTarget.Output, -4000, 2)
	if err != nil {
		msg := fmt.Sprintf("cannot tail the log file with id %d", params.ID)
		rsp := services.NewGetLogTailDefault(http.StatusInternalServerError).WithPayload(&models.APIError{
			Message: &msg,
		})
		return rsp
	}

	// Everything ok. Return the response.
	tail := &models.LogTail{
		AgentAddress:    dbLogTarget.Daemon.App.Machine.Address,
		AgentPort:       dbLogTarget.Daemon.App.Machine.AgentPort,
		AppID:           dbLogTarget.Daemon.App.ID,
		AppType:         dbLogTarget.Daemon.App.Type,
		LogTargetOutput: dbLogTarget.Output,
		Contents:        contents,
	}
	rsp := services.NewGetLogTailOK().WithPayload(tail)

	return rsp
}
