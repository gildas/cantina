package main

import (
	"context"
	"net/http"
	"net/url"

	"github.com/gildas/go-errors"
)

type key int
const contextKey key = iota

type Config struct {
	MetaRoot    string
	StorageRoot string
	StorageURL  url.URL
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
