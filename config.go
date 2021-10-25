package main

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

type key int
const contextKey key = iota

type Config struct {
	MetaRoot    string
	PurgeAfter  time.Duration
	StorageRoot string
	StorageURL  url.URL
}

func (config Config) WithRequest(r *http.Request) Config {
	log := logger.Must(logger.FromContext(r.Context()))
	newConfig := config
	if value := r.FormValue("purgeAfter"); len(value) > 0 {
		purgeAfter, err := core.ParseDuration(value)
		if err != nil {
			log.Errorf("Failed to parse duration from form value purgeAfter (%s)", r.FormValue("purgeAfter"), err)
		} else {
			log.Infof("File will be purge in approximatedly %s on %s", purgeAfter, time.Now().UTC().Add(purgeAfter))
			newConfig.PurgeAfter = purgeAfter
		}
	} else if value := r.FormValue("purgeOn"); len(value) > 0 {
		purgeOn, err := core.ParseTime(value)
		if err != nil {
		} else {
			purgeAfter := time.Until(purgeOn.AsTime())
			if purgeAfter > 0 {
				log.Infof("File will be purge on %s in (%s)", purgeOn, purgeAfter)
				newConfig.PurgeAfter = purgeAfter
			}
		}
	}
	return newConfig
}

// ConfigFromContext retrieves the Config from the given Context
func ConfigFromContext(context context.Context) (Config, error) {
	if config, ok := context.Value(contextKey).(Config); ok {
		return config, nil
	}
	return Config{}, errors.ArgumentMissing.With("config")
}

// ToContext stores the Config to the given Context
func (config Config) ToContext(parent context.Context) context.Context {
	return context.WithValue(parent, contextKey, config)
}

// HttpHandler middleware for storing Config in routers
func (config Config) HttpHandler() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(config.ToContext(r.Context())))
		})
	}
}
