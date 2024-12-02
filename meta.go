package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

type MetaInformation struct {
	Filename      string     `json:"filename"`
	CreatedAt     time.Time  `json:"-"`
	DeleteAt      *time.Time `json:"-"` // Can be nil
	MimeType      string     `json:"mimeType"`
	Size          uint64     `json:"size"`
	MaxDownloads  uint64     `json:"maxDownloads"`
	DownloadCount uint64     `json:"downloadCount"`
	Password      string     `json:"password,omitempty"`
	config        Config
}

// CreateMetaInformation creates a meta information
//
// a file is created in the meta folder
func CreateMetaInformation(context context.Context, config Config, filename string, mimetype string, size uint64) (MetaInformation, error) {
	metadata := MetaInformation{
		CreatedAt: time.Now().UTC(),
		Filename:  filename,
		MimeType:  mimetype,
		Size:      size,
		Password:  config.Password,
		config:    config,
	}
	if config.PurgeAfter > 0 {
		deleteAt := metadata.CreatedAt.Add(config.PurgeAfter)
		metadata.DeleteAt = &deleteAt
	}
	err := metadata.Save(context)
	if err != nil {
		return MetaInformation{}, err
	}
	return metadata, nil
}

// FindMetaInformation find MetaInformation about the given filename or assing a new one
func FindMetaInformation(context context.Context, config Config, filename string) *MetaInformation {
	log := logger.Must(logger.FromContext(context)).Child("meta", "find", "filename", filename)
	metadata := &MetaInformation{}

	if payload, err := os.ReadFile(MetaInformation{Filename: filename, config: config}.Path()); err == nil {
		log.Debugf("Found metadata for %s: %s", filename, string(payload))
		err = json.Unmarshal(payload, &metadata)
		if err == nil {
			metadata.config = config
			return metadata
		} else {
			log.Errorf("Failed to unmarshal metadata for %s", filename, err)
		}
	}
	return &MetaInformation{
		Filename: filename,
		config:   config,
	}
}

// Update updates the MetaInformation
func (metadata *MetaInformation) Update(context context.Context, update MetaInformation) error {
	log := logger.Must(logger.FromContext(context)).Child("meta", "update", "filename", metadata.Filename)

	if len(update.MimeType) > 0 && update.MimeType != metadata.MimeType {
		log.Infof("Updating MimeType from %s to %s", metadata.MimeType, update.MimeType)
		metadata.MimeType = update.MimeType
	}
	if len(update.Password) > 0 {
		log.Infof("Updating Password from %s to %s", metadata.Password, update.Password)
		metadata.Password = update.Password
	}
	if update.DeleteAt != nil && (metadata.DeleteAt == nil || metadata.DeleteAt != update.DeleteAt) {
		log.Infof("Updating DeleteAt from %s to %s", metadata.DeleteAt, update.DeleteAt)
		metadata.DeleteAt = update.DeleteAt
	}
	if update.MaxDownloads > 0 && metadata.MaxDownloads != update.MaxDownloads {
		log.Infof("Updating MaxDownloads from %d to %d", metadata.MaxDownloads, update.MaxDownloads)
		metadata.MaxDownloads = update.MaxDownloads
	}
	return metadata.Save(context)
}

// Save saves the MetaInformation
func (metadata MetaInformation) Save(context context.Context) error {
	if len(metadata.Password) > 0 && !strings.HasPrefix(metadata.Password, "!ENC!") {
		hash := sha256.New()
		hash.Write([]byte(metadata.Password))
		metadata.Password = "!ENC!" + base64.StdEncoding.EncodeToString(hash.Sum(nil))
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(metadata.Path(), payload, 0600)
}

// Delete deletes the file holding the MetaInformation
func (metadata MetaInformation) Delete(context context.Context) error {
	err := os.Remove(metadata.Path())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// DeleteContent deletes all files handled by this MetaInformation
func (metadata MetaInformation) DeleteContent(context context.Context) error {
	destination := filepath.Join(metadata.config.StorageRoot, metadata.Filename)
	if err := os.Remove(destination); err != nil {
		return err
	}
	// delete the thumbnail (if any)
	basename := strings.TrimSuffix(filepath.Base(metadata.Filename), filepath.Ext(metadata.Filename))
	destination = filepath.Join(metadata.config.StorageRoot, filepath.Dir(metadata.Filename), basename+"-thumbnail.png")
	if err := os.Remove(destination); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Path tells the Path of the file holding the MetaInformation
func (metadata MetaInformation) Path() string {
	return filepath.Join(metadata.config.MetaRoot, metadata.Filename+".json")
}

// Authenticate tells if the given password is correct
func (metadata MetaInformation) Authenticate(password string) bool {
	hash := sha256.New()
	hash.Write([]byte(password))
	hashed := "!ENC!" + base64.StdEncoding.EncodeToString(hash.Sum(nil))
	return metadata.Password == hashed
}

// IncrementDownloadCount increments the download count
//
// It also saves the MetaInformation. If the MaxDownloads is reached, it marks the MetaInformation for deletion
func (metadata *MetaInformation) IncrementDownloadCount(context context.Context) error {
	log := logger.Must(logger.FromContext(context)).Child("meta", "increment", "filename", metadata.Filename)
	metadata.DownloadCount++
	if metadata.MaxDownloads > 0 && metadata.DownloadCount >= metadata.MaxDownloads {
		log.Infof("Download count reached the limit (%d)", metadata.MaxDownloads)
		deleteAt := time.Now().UTC()
		metadata.DeleteAt = &deleteAt
	}
	return metadata.Save(context)
}

// Redact redacts the MetaInformation
func (metadata MetaInformation) Redact() any {
	redacted := metadata
	if len(redacted.Password) > 0 {
		redacted.Password = logger.RedactWithHash(redacted.Password)
	}
	return redacted
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
		CreatedAt core.Time `json:"createdAt"`
	}
	if err = json.Unmarshal(payload, &inner); err != nil {
		return errors.JSONUnmarshalError.Wrap(err)
	}
	*metadata = MetaInformation(inner.surrogate)
	metadata.CreatedAt = inner.CreatedAt.AsTime()

	var values map[string]any

	if err = json.Unmarshal(payload, &values); err != nil {
		return errors.JSONUnmarshalError.Wrap(err)
	}

	if metadata.DeleteAt, err = unmarshalTime(values, "deleteAt", "purgeAt", "purgeOn"); err != nil && !errors.Is(err, errors.NotFound) {
		return errors.JSONUnmarshalError.Wrap(err)
	}

	if metadata.DeleteAt == nil {
		deleteIn, err := unmarshalDuration(values, "deleteIn", "deleteAfter", "purgeIn", "purgeAfter")
		if err != nil && !errors.Is(err, errors.NotFound) {
			return errors.JSONUnmarshalError.Wrap(err)
		}
		if deleteIn != nil {
			deleteAt := time.Now().UTC().Add(time.Duration(*deleteIn))
			metadata.DeleteAt = &deleteAt
		}
	}
	return nil
}

func unmarshalTime(values map[string]any, name ...string) (*time.Time, error) {
	for _, key := range name {
		if value, ok := values[key]; ok {
			return parseTime(value)
		}
	}
	return nil, errors.NotFound.With("")
}

func parseTime(value any) (*time.Time, error) {
	if value == nil {
		return nil, errors.NotFound.With("value")
	}
	if stringValue, ok := value.(string); ok {
		timeValue, err := core.ParseTime(stringValue)
		if err == nil {
			timeValue := time.Time(timeValue)
			return &timeValue, nil
		}
		return nil, err
	}
	// Support epoch?
	return nil, errors.NotFound.With("value")
}

func unmarshalDuration(values map[string]any, name ...string) (*time.Duration, error) {
	for _, key := range name {
		if value, ok := values[key]; ok {
			return parseDuration(value)
		}
	}
	return nil, errors.NotFound.With("")
}

func parseDuration(value any) (*time.Duration, error) {
	if value == nil {
		return nil, errors.NotFound.With("value")
	}
	if stringValue, ok := value.(string); ok {
		durationValue, err := core.ParseDuration(stringValue)
		if err == nil {
			durationValue := time.Duration(durationValue)
			return &durationValue, nil
		}
		return nil, err
	}
	// Support epoch?
	return nil, errors.NotFound.With("value")
}
