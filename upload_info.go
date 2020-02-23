package main

import (
	"encoding/json"
	"net/url"

	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
)

type UploadInfo struct {
	ContentURL   *url.URL `json:"-"`
	ThumbnailURL *url.URL `json:"-"`
	MimeType     string   `json:"mimeType"`
	Size         int64    `json:"size"`
}

func (info UploadInfo) MarshalJSON() ([]byte, error) {
	type surrogate UploadInfo
	data, err := json.Marshal(struct {
		surrogate
		C *core.URL `json:"contentUrl"`
		T *core.URL `json:"thumnailUrl,omitempty"`
	}{
		C: (*core.URL)(info.ContentURL),
		T: (*core.URL)(info.ThumbnailURL),
		surrogate: surrogate(info),
	})
	return data, errors.JSONMarshalError.Wrap(err)
}