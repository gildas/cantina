package main

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
)

func FilesRoutes(router *mux.Router, apiRoot, storageRoot string, storageURL *url.URL, log *logger.Logger) {
	router.Methods("POST").Path(apiRoot + "/files").Handler(log.HttpHandler()(CreateFileHandler(storageRoot, storageURL)))
}

func CreateFileHandler(storageRoot string, storageURL *url.URL) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context()))
		log.Debugf("Request Headers: %#v", r.Header)

		err := r.ParseMultipartForm(500 * 1024 * 1024)
		if err != nil {
			log.Errorf("Failed to parse Multipart form", err)
			core.RespondWithError(w, http.StatusBadRequest, err)
			return
		}

		log.Infof("Creating a File in %s", storageRoot)
		reader, header, err := r.FormFile("file")
		if err != nil {
			log.Errorf("Failed to get form field \"file\"", err)
			core.RespondWithError(w, http.StatusBadRequest, err)
			return
		}
		defer reader.Close()

		destination := path.Join(storageRoot, header.Filename)
		log.Debugf("Writing %d bytes to %s", header.Size, destination)
		log.Debugf("MIME: %#v", header.Header.Get("Content-Type"))
		writer, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			log.Errorf("Failed to open file %s for writing", destination, err)
			core.RespondWithError(w, http.StatusInternalServerError, err)
			return
		}
		defer writer.Close()

		written, err := io.Copy(writer, reader)
		if err != nil {
			log.Errorf("Failed to write file", err)
			core.RespondWithError(w, http.StatusInternalServerError, err)
			return
		}
		log.Infof("Written %d bytes to %s", written, destination)

		uploadInfo, err := UploadInfoFrom(log, storageURL, destination, header.Filename, header.Header.Get("Content-Type"), written)
		if err != nil {
			log.Errorf("Failed to build upload info", err)
			core.RespondWithError(w, http.StatusInternalServerError, err)
			return
		}

		core.RespondWithJSON(w, http.StatusOK, uploadInfo)
	})
}