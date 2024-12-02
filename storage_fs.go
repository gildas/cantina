package main

import (
	"context"
	"net/http"
	"os"
	"strings"

	"github.com/gildas/go-logger"
)

// StorageFileSystem represents an http.FileSystem tailored to our needs
type StorageFileSystem struct {
	http.FileSystem
	log    *logger.Logger
	config Config
}

// StorageFile represents an http.File part of our StorageFileSystem
type StorageFile struct {
	http.File
}

// Readdir reads the concents of the directory associated with the StorageFile
//
// implements http.File
func (fs StorageFile) Readdir(n int) (fis []os.FileInfo, err error) {
	files, err := fs.File.Readdir(n)
	for _, file := range files {
		if !strings.HasPrefix(file.Name(), ".") {
			fis = append(fis, file)
		}
	}
	return
}

// IsValid tells if a filename is valid and can be downloaded
//
// Basically we do not want to server dot files (at least)
func (fs StorageFileSystem) IsValid(filename string) bool {
	if filename == "/" {
		return false
	}
	parts := strings.Split(filename, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return false
		}
	}
	return true
}

// Open the named file for reading
//
// # If the file is not valid, os.ErrPermission is returned
//
// implements http.FileSystem
func (fs StorageFileSystem) Open(name string) (http.File, error) {
	if !fs.IsValid(name) {
		return nil, os.ErrPermission
	}
	file, err := fs.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	ctx := fs.log.ToContext(context.Background())
	metaInformation := FindMetaInformation(ctx, fs.config, name)
	if err := metaInformation.IncrementDownloadCount(ctx); err != nil {
		return nil, err
	}
	return StorageFile{file}, err
}
