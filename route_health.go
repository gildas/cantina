package main

import (
	"fmt"
	"net/http"
	"sync/atomic"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
)

// HealthHTTP contains the health of the web server
// sync/atomic should be used to read/write here
var HealthHTTP int32

// HealthRoutes fills the router with routes for health
func HealthRoutes(router *mux.Router) {
	router.Methods("GET").Path("/liveness").Handler(healthLivenessHandler())
	router.Methods("GET").Path("/readiness").Handler(healthReadinessHandler())
}

// healthLivenessHandler processes requests that check the health of this app (e.g.: Kubernetes)
func healthLivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(core.GetEnvAsString("TRACE_PROBE", "")) > 0 {
			log, err := logger.FromContext(r.Context())
			if err == nil {
				log.Topic("probe").Scope("liveness").Tracef("It is alive!")
			}
		}
		core.RespondWithJSON(w, http.StatusOK, struct{}{})
	})
}

// healthReadinessHandler processes requests that check if this app is ready to process data (i.e.: Kubernetes)
func healthReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var logit = len(core.GetEnvAsString("TRACE_PROBE", "")) > 0
		var log *logger.Logger

		if logit {
			var err error

			if log, err = logger.FromContext(r.Context()); err != nil {
				Log.Errorf("Failed to retrieve logger from Context", err)
				log = logger.Create(APP, &logger.NilStream{})
			} else {
				log = log.Child("probe", "readiness")
			}
		} else {
			log = logger.Create(APP, &logger.NilStream{})
		}

		if atomic.LoadInt32(&HealthHTTP) == 0 {
			log.Errorf("WebServer not ready")
			core.RespondWithError(w, http.StatusServiceUnavailable, fmt.Errorf("WebServer Not Ready"))
			return
		}

		log.Tracef("It is ready!")
		core.RespondWithJSON(w, http.StatusOK, struct{}{})
	})
}
