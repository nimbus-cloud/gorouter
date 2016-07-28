package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/cloudfoundry-incubator/routing-api/authentication"
	"github.com/cloudfoundry-incubator/routing-api/db"
	"github.com/pivotal-golang/lager"
)

type TcpRouteMappingsHandler struct {
	tokenValidator authentication.TokenValidator
	validator      RouteValidator
	db             db.DB
	logger         lager.Logger
}

func NewTcpRouteMappingsHandler(tokenValidator authentication.TokenValidator, validator RouteValidator, database db.DB, logger lager.Logger) *TcpRouteMappingsHandler {
	return &TcpRouteMappingsHandler{
		tokenValidator: tokenValidator,
		validator:      validator,
		db:             database,
		logger:         logger,
	}
}

func (h *TcpRouteMappingsHandler) List(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("list-tcp-route-mappings")

	err := h.tokenValidator.DecodeToken(req.Header.Get("Authorization"), RoutingRoutesReadScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}
	routes, err := h.db.ReadTcpRouteMappings()
	if err != nil {
		handleDBCommunicationError(w, err, log)
		return
	}
	encoder := json.NewEncoder(w)
	encoder.Encode(routes)
}

func (h *TcpRouteMappingsHandler) Upsert(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("create-tcp-route-mappings")
	decoder := json.NewDecoder(req.Body)

	var tcpMappings []db.TcpRouteMapping
	err := decoder.Decode(&tcpMappings)
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}

	log.Info("request", lager.Data{"tcp_mapping_creation": tcpMappings})

	err = h.tokenValidator.DecodeToken(req.Header.Get("Authorization"), RoutingRoutesWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	apiErr := h.validator.ValidateCreateTcpRouteMapping(tcpMappings)
	if apiErr != nil {
		handleProcessRequestError(w, apiErr, log)
		return
	}

	for _, tcpMapping := range tcpMappings {
		err = h.db.SaveTcpRouteMapping(tcpMapping)
		if err != nil {
			handleDBCommunicationError(w, err, log)
			return
		}
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *TcpRouteMappingsHandler) Delete(w http.ResponseWriter, req *http.Request) {
	log := h.logger.Session("delete-tcp-route-mappings")
	decoder := json.NewDecoder(req.Body)

	var tcpMappings []db.TcpRouteMapping
	err := decoder.Decode(&tcpMappings)
	if err != nil {
		handleProcessRequestError(w, err, log)
		return
	}

	log.Info("request", lager.Data{"tcp_mapping_deletion": tcpMappings})

	err = h.tokenValidator.DecodeToken(req.Header.Get("Authorization"), RoutingRoutesWriteScope)
	if err != nil {
		handleUnauthorizedError(w, err, log)
		return
	}

	apiErr := h.validator.ValidateDeleteTcpRouteMapping(tcpMappings)
	if apiErr != nil {
		handleProcessRequestError(w, apiErr, log)
		return
	}

	for _, tcpMapping := range tcpMappings {
		err = h.db.DeleteTcpRouteMapping(tcpMapping)
		if err != nil {
			if dberr, ok := err.(db.DBError); !ok || dberr.Type != db.KeyNotFound {
				handleDBCommunicationError(w, err, log)
				return
			}
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
