package main

import (
	"net/http"
	"os"
	"strings"
)

// StorageFileSystem represents an http.FileSystem tailored to our needs
type StorageFileSystem struct {
	http.FileSystem
}

// StorageFile represents an http.File part of our StorageFileSystem
type StorageFile struct {
	http.File
}

// Readdir reads the concents of the directory associated with the StorageFile
//
// implements http.File
func (sf StorageFile) Readdir(n int) (fis []os.FileInfo, err error) {
	files, err := sf.File.Readdir(n)
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
// If the file is not valid, os.ErrPermission is returned
//
// implements http.FileSystem
func (sfs StorageFileSystem) Open(name string) (http.File, error) {
	if !sfs.IsValid(name) {
		return nil, os.ErrPermission
	}
	file, err := sfs.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	return StorageFile{file}, err
}
