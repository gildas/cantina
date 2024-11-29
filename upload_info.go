package main

import (
	"context"
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"github.com/gildas/go-core"
	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

type UploadInfo struct {
	ContentURL   *url.URL      `json:"-"`
	ThumbnailURL *url.URL      `json:"-"`
	Duration     time.Duration `json:"-"`
	DeleteAt     *time.Time    `json:"-"`
	MimeType     string        `json:"mimeType"`
	Size         uint64        `json:"size"`
	Password     string        `json:"password,omitempty"`
}

func UploadInfoFrom(context context.Context, storageURL *url.URL, path string, metadata MetaInformation) (*UploadInfo, error) {
	log := logger.Must(logger.FromContext(context)).Child("uploadinfo", "create", "filename", metadata.Filename)
	var err error
	info := &UploadInfo{
		MimeType: metadata.MimeType,
		Size:     metadata.Size,
		DeleteAt: metadata.DeleteAt,
	}

	info.ContentURL, err = storageURL.Parse(metadata.Filename)
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasPrefix(metadata.MimeType, "image"):
		// TODO: If the file is an image, calculate a thumbnail
		thumbnail, err := info.getThumbnail(path)
		if err != nil {
			log.Warnf("Failed to create a thumbnail, we will use a default icon, Error: %s", err)
			info.ThumbnailURL, _ = url.Parse("https://cdn2.iconfinder.com/data/icons/freecns-cumulus/16/519587-084_Photo-64.png")
		} else {
			info.ThumbnailURL, _ = storageURL.Parse(thumbnail)
		}
	case strings.HasPrefix(metadata.MimeType, "audio"):
		info.ThumbnailURL, _ = url.Parse("https://cdn1.iconfinder.com/data/icons/ios-11-glyphs/30/circled_play-64.png")
		// TODO: If the file is an audio, calculate its duration in seconds
	case strings.HasPrefix(metadata.MimeType, "video"):
		info.ThumbnailURL, _ = url.Parse("https://cdn2.iconfinder.com/data/icons/flat-ui-icons-24-px/24/video-24-64.png")
		// TODO: If the file is a video?!? thumbnail (with an icon in the middle?!?), duration?
	case metadata.MimeType == "application/pdf":
		fallthrough
	default:
		info.ThumbnailURL, _ = url.Parse("https://cdn1.iconfinder.com/data/icons/material-core/19/file-download-64.png")
	}
	return info, nil
}

func (info UploadInfo) getThumbnail(path string) (string, error) {
	original, err := imaging.Open(path)
	if err != nil {
		return "", nil
	}
	thumbnail := imaging.Thumbnail(original, 128, 128, imaging.CatmullRom)
	thumbnailName := strings.Builder{}
	thumbnailName.WriteString(strings.Split(filepath.Base(path), filepath.Ext(path))[0]) // we want the base name without the extension
	thumbnailName.WriteString("-thumbnail.png")
	err = imaging.Save(thumbnail, filepath.Join(filepath.Dir(path), thumbnailName.String()))
	return thumbnailName.String(), err
}

func (info UploadInfo) MarshalJSON() ([]byte, error) {
	type surrogate UploadInfo
	data, err := json.Marshal(struct {
		surrogate
		ContentURL   *core.URL     `json:"contentUrl"`
		ThumbnailURL *core.URL     `json:"thumbnailUrl,omitempty"`
		Duration     core.Duration `json:"duration,omitempty"`
		DeleteAt     *core.Time    `json:"deleteAt,omitempty"`
	}{
		ContentURL:   (*core.URL)(info.ContentURL),
		ThumbnailURL: (*core.URL)(info.ThumbnailURL),
		Duration:     (core.Duration)(info.Duration),
		DeleteAt:     (*core.Time)(info.DeleteAt),
		surrogate:    surrogate(info),
	})
	return data, errors.JSONMarshalError.Wrap(err)
}
