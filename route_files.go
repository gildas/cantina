package main

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
)

func FilesRoutes(router *mux.Router, storageRoot string, storageURL *url.URL) {
	filesRouter := router.PathPrefix("/files").Subrouter()

	filesRouter.Methods(http.MethodPost).Handler(createFileHandler(storageRoot, storageURL))
	filesRouter.Methods(http.MethodDelete).Path("/{filename}").Handler(deleteFileHandler(storageRoot, storageURL))
}

func createFileHandler(storageRoot string, storageURL *url.URL) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Must(logger.FromContext(r.Context()))
		log.Debugf("Request Headers: %#v", r.Header)

		r.Body = http.MaxBytesReader(w, r.Body, 5 << 30) // uploads are limited to 5GB
		err := r.ParseMultipartForm(5 << 20) // we can deal with 5MB in RAM
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

func deleteFileHandler(storageRoot string, storageURL *url.URL) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request)  {
		log := logger.Must(logger.FromContext(r.Context()))
		log.Debugf("Request Headers: %#v", r.Header)

		params := mux.Vars(r)
		filename := params["filename"]
		if len(filename) == 0 {
			log.Errorf("Missing Filename from path")
			core.RespondWithError(w, http.StatusBadRequest, errors.ArgumentMissing.With("filename"))
			return
		}

		extension := path.Ext(filename)
		basename := strings.TrimSuffix(path.Base(filename), extension)
		destination := path.Join(storageRoot, path.Dir(filename), basename + "-thumbnail" + extension)
		if err := os.Remove(destination); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				if errors.Is(err, fs.ErrPermission) {
					log.Errorf("Not enough permission to delete file %s", destination, err)
				}
				log.Errorf("Error while deleting %s", destination, err)
			}
			log.Errorf("Error while deleting %s", destination, err)
			core.RespondWithError(w, http.StatusInternalServerError, errors.UnknownError.With(fmt.Sprintf("Cannot delete %s, error: %s", filename, err)))
			return
		}

		destination = path.Join(storageRoot, filename)
		if err := os.Remove(destination); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				log.Errorf("File %s was not found", filename, err)
				core.RespondWithError(w, http.StatusNotFound, errors.NotFound.With("file", filename))
				return
			}
			if errors.Is(err, fs.ErrPermission) {
				log.Errorf("Not enough permission to delete file %s", filename, err)
				core.RespondWithError(w, http.StatusForbidden, errors.HTTPForbidden.With(filename))
			}
			log.Errorf("Error while deleting %s", destination, err)
			core.RespondWithError(w, http.StatusInternalServerError, errors.UnknownError.With(filename))
			return
		}
		log.Infof("File %s was deleted successfully", filename)
		w.WriteHeader(http.StatusNoContent)
	})
}
