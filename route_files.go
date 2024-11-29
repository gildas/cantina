package main

import (
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
)

func FilesRoutes(router *mux.Router) {
	filesRouter := router.PathPrefix("/files").Subrouter()

	filesRouter.Methods(http.MethodPost).HandlerFunc(createFileHandler)
	filesRouter.Methods(http.MethodPatch).Path("/{filename}").HandlerFunc(patchFileHandler)
	filesRouter.Methods(http.MethodDelete).Path("/{filename}").HandlerFunc(deleteFileHandler)
}

func createFileHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Must(logger.FromContext(r.Context()))
	config := core.Must(ConfigFromContext(r.Context()))
	log.Debugf("Request Headers: %#v", r.Header)

	r.Body = http.MaxBytesReader(w, r.Body, 5<<30) // uploads are limited to 5GB
	err := r.ParseMultipartForm(5 << 20)           // we can deal with 5MB in RAM
	if err != nil {
		log.Errorf("Failed to parse Multipart form", err)
		core.RespondWithError(w, http.StatusBadRequest, err)
		return
	}

	log.Infof("Creating a File in %s", config.StorageRoot)
	reader, header, err := r.FormFile("file")
	if err != nil {
		log.Errorf("Failed to get form field \"file\"", err)
		core.RespondWithError(w, http.StatusBadRequest, err)
		return
	}
	defer reader.Close()
	filename := filepath.Clean(header.Filename)
	log = log.Record("filename", filename)
	context := log.ToContext(r.Context())

	destination := path.Join(config.StorageRoot, filename)
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

	metadata, err := CreateMetaInformation(context, config.WithRequest(r), filename, header.Header.Get("Content-Type"), uint64(written))
	if err != nil {
		log.Errorf("Failed to build metadata info", err)
		core.RespondWithError(w, http.StatusInternalServerError, err)
		return
	}

	uploadInfo, err := UploadInfoFrom(context, &config.StorageURL, destination, metadata)
	if err != nil {
		log.Errorf("Failed to build upload info", err)
		core.RespondWithError(w, http.StatusInternalServerError, err)
		return
	}

	core.RespondWithJSON(w, http.StatusOK, uploadInfo)
}

func patchFileHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Must(logger.FromContext(r.Context()))
	config := core.Must(ConfigFromContext(r.Context()))
	log.Debugf("Request Headers: %#v", r.Header)

	params := mux.Vars(r)
	filename := params["filename"]
	if len(filename) == 0 {
		log.Errorf("Missing Filename from path")
		core.RespondWithError(w, http.StatusBadRequest, errors.ArgumentMissing.With("filename"))
		return
	}
	filename = filepath.Clean(filename)
	log = log.Record("filename", filename)
	context := log.ToContext(r.Context())

	metadata := FindMetaInformation(context, config, filename)
	log.Record("metadata", metadata).Infof("Loaded metadata for %s", filename)

	// Analyze the body
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Errorf("Failed to read the request body: %s", err)
		core.RespondWithError(w, http.StatusBadRequest, err)
		return
	}
	log.Scope("payload").Tracef("Request Body: %s", body)

	var update MetaInformation

	if err := json.Unmarshal(body, &update); err != nil {
		log.Errorf("Failed to unmarshal the request body", err)
		core.RespondWithError(w, http.StatusBadRequest, err)
		return
	}
	log.Record("update", update).Debugf("Metadata Unmarshaled")

	if err := metadata.Update(context, update); err != nil {
		log.Errorf("Failed to update meta information", err)
		core.RespondWithError(w, http.StatusInternalServerError, errors.UnknownError.With(filename))
		return
	}

	log.Infof("File %s was updated successfully", filename)
	w.WriteHeader(http.StatusNoContent)
}

func deleteFileHandler(w http.ResponseWriter, r *http.Request) {
	log := logger.Must(logger.FromContext(r.Context()))
	config := core.Must(ConfigFromContext(r.Context()))
	log.Debugf("Request Headers: %#v", r.Header)

	params := mux.Vars(r)
	filename := params["filename"]
	if len(filename) == 0 {
		log.Errorf("Missing Filename from path")
		core.RespondWithError(w, http.StatusBadRequest, errors.ArgumentMissing.With("filename"))
		return
	}
	filename = filepath.Clean(filename)
	log = log.Record("filename", filename)
	context := log.ToContext(r.Context())

	metadata := FindMetaInformation(context, config, filename)
	if err := metadata.DeleteContent(context); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Errorf("File %s was not found", filename, err)
			core.RespondWithError(w, http.StatusNotFound, errors.NotFound.With("file", filename))
			return
		}
		if errors.Is(err, fs.ErrPermission) {
			log.Errorf("Not enough permission to delete file %s", filename, err)
			core.RespondWithError(w, http.StatusForbidden, errors.HTTPForbidden.With(filename))
		}
		log.Errorf("Error while deleting %s", filename, err)
		core.RespondWithError(w, http.StatusInternalServerError, errors.UnknownError.With(filename))
		return
	}

	if err := metadata.Delete(context); err != nil {
		log.Errorf("Failed to delete meta information", err)
	}

	log.Infof("File %s was deleted successfully", filename)
	w.WriteHeader(http.StatusNoContent)
}
