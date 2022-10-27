package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

type Authority struct {
	AuthRoot string
}

// HttpHandler is the middleware to protect a route
func (auth Authority) HttpHandler() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := logger.Must(logger.FromContext(r.Context())).Child("auth", nil)

			key := r.Header.Get("X-Key")
			if len(key) == 0 {
				// TODO: try to read the key from application/x-www-form-urlencoded and multipart/form-data
				key = r.URL.Query().Get("key")
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

			// TODO: Read the content of that file to get the user info (file: username.json)

			next.ServeHTTP(w, r)
		})
	}
}