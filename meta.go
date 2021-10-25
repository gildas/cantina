package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
)

type MetaInformation struct {
	Filename  string     `json:"filename"`
	CreatedAt time.Time  `json:"-"`
	DeleteAt  *time.Time `json:"-"` // Can be nil
	MimeType  string     `json:"mimeType"`
	Size      uint64     `json:"size"`
	config    Config
}

// CreateMetaInformation creates a meta information
//
// a file is created in the meta folder
func CreateMetaInformation(config Config, filename string, mimetype string, size uint64) (MetaInformation, error) {
	metadata := MetaInformation{
		CreatedAt: time.Now().UTC(),
		Filename:  filename,
		MimeType:  mimetype,
		Size:      size,
		config:    config,
	}
	if config.PurgeAfter > 0 {
		deleteAt := metadata.CreatedAt.Add(config.PurgeAfter)
		metadata.DeleteAt = &deleteAt
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return MetaInformation{}, err
	}
	err = os.WriteFile(filepath.Join(config.MetaRoot, filename + ".json"), payload, 0666)
	if err != nil {
		return MetaInformation{}, err
	}
	return metadata, nil
}

// FindMetaInformation find MetaInformation about the given filename
func FindMetaInformation(config Config, filename string) (metadata MetaInformation, err error) {
	payload, err := os.ReadFile(MetaInformation{Filename: filename, config: config}.Path())
	if err != nil {
		return metadata, errors.NotFound.With(filename).(errors.Error).Wrap(err)
	}
	err = json.Unmarshal(payload, &metadata)
	if err == nil {
		metadata.config = config
	}
	return
}

// Delete deletes the file holding the MetaInformation
func (metadata MetaInformation) Delete() error {
	return os.Remove(metadata.Path());
}

// Path tells the Path of the file holding the MetaInformation
func (metadata MetaInformation) Path() string {
	return filepath.Join(metadata.config.MetaRoot, metadata.Filename + ".json")
}

// MarshalJSON marshals this into JSON
func (metadata MetaInformation) MarshalJSON() ([]byte, error) {
	type surrogate MetaInformation
	data, err := json.Marshal(struct {
		surrogate
		CreatedAt core.Time  `json:"createdAt"`
		DeleteAt  *core.Time `json:"deleteAt,omitempty"`
	}{
		surrogate: surrogate(metadata),
		CreatedAt: (core.Time)(metadata.CreatedAt),
		DeleteAt:  (*core.Time)(metadata.DeleteAt),
	})
	return data, errors.JSONMarshalError.Wrap(err)
}

func (metadata *MetaInformation) UnmarshalJSON(payload []byte) (err error) {
	type surrogate MetaInformation
	var inner struct {
		surrogate
		CreatedAt core.Time  `json:"createdAt"`
		DeleteAt  *core.Time `json:"deleteAt,omitempty"`
	}
	if err = json.Unmarshal(payload, &inner); err != nil {
		return errors.JSONUnmarshalError.Wrap(err)
	}
	*metadata = MetaInformation(inner.surrogate)
	metadata.CreatedAt = inner.CreatedAt.AsTime()
	metadata.DeleteAt  = (*time.Time)(inner.DeleteAt)
	return
}
