package main

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

type Authority struct {
	AuthRoot string
}

// Middleware is the middleware to protect a route
func (auth Authority) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.Must(logger.FromContext(r.Context())).Child("auth", nil)

			var key string

			authorization := r.Header.Get("Authorization")
			if len(authorization) > 0 {
				parts := strings.Split(authorization, " ")
				if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
					log.Errorf("HTTP Request carries an invalid Authorization header: %s", authorization)
					core.RespondWithError(w, http.StatusForbidden, errors.ArgumentInvalid.With("Authorization", authorization))
					return
				}
				key = parts[1]
			}

			if len(key) == 0 {
				key = r.Header.Get("X-Key")
				if len(key) == 0 {
					// TODO: try to read the key from application/x-www-form-urlencoded and multipart/form-data
					key = r.URL.Query().Get("key")
				}
			}

			if len(key) == 0 {
				log.Errorf("HTTP Request does not carry a key in its parameters or headers")
				core.RespondWithError(w, http.StatusForbidden, errors.ArgumentMissing.With("X-Key or key"))
				return
			}

			// Sanitizing the key
			key = filepath.Clean(key)
			if strings.ContainsAny(key, "\\/:<>|?*") {
				log.Errorf("HTTP Request carries an invalid key in its parameters or headers: %s", key)
				core.RespondWithError(w, http.StatusForbidden, errors.ArgumentInvalid.With("X-Key or key", key))
				return
			}

			authFile, err := os.Open(filepath.Join(auth.AuthRoot, key))
			if err != nil {
				log.Errorf("Key %s does not exist, not authorized", key, err)
				core.RespondWithError(w, http.StatusForbidden, errors.HTTPUnauthorized)
				return
			}
			defer authFile.Close()

			next.ServeHTTP(w, r)
		})
	}
}

// DownloadMiddleware is the middleware to protect a download route
func (auth Authority) DownloadMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.Must(logger.FromContext(r.Context())).Child("auth", nil)

			// Open the metadata file
			// config := core.Must(ConfigFromContext(r.Context()))
			config := Config{MetaRoot: auth.AuthRoot}

			filename := path.Base(filepath.Clean(r.URL.Path))
			log.Infof("Requested file: %s", filename)
			metadata := FindMetaInformation(r.Context(), config, filename)
			log.Record("metadata", metadata).Infof("Loaded metadata for %s", filename)

			if metadata.Password != "" {
				var key string

				log.Infof("File %s is protected by a password", filename)
				authorization := r.Header.Get("Authorization")
				if len(authorization) > 0 {
					parts := strings.Split(authorization, " ")
					if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
						log.Errorf("HTTP Request carries an invalid Authorization header: %s", authorization)
						core.RespondWithError(w, http.StatusForbidden, errors.HTTPUnauthorized)
						return
					}
					key = parts[1]
				}

				if len(key) == 0 {
					key = r.Header.Get("X-Key")
					if len(key) == 0 {
						// TODO: try to read the key from application/x-www-form-urlencoded and multipart/form-data
						key = r.URL.Query().Get("key")
					}
				}

				if len(key) == 0 {
					log.Errorf("HTTP Request does not carry authorization nor a key in its parameters or headers")
					core.RespondWithError(w, http.StatusForbidden, errors.HTTPUnauthorized)
					return
				}

				// Sanitizing the key
				key = filepath.Clean(key)
				if strings.ContainsAny(key, "\\/:<>|?*") {
					log.Errorf("HTTP Request carries an invalid key in its parameters or headers: %s", key)
					core.RespondWithError(w, http.StatusForbidden, errors.HTTPUnauthorized)
					return
				}

				if !metadata.Authenticate(key) {
					log.Errorf("Key %s is not authorized to download %s", key, filename)
					core.RespondWithError(w, http.StatusForbidden, errors.HTTPUnauthorized)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
