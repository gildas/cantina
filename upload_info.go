package main

import (
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
	ContentURL   *url.URL       `json:"-"`
	ThumbnailURL *url.URL       `json:"-"`
	Duration     time.Duration  `json:"-"`
	MimeType     string         `json:"mimeType"`
	Size         int64          `json:"size"`
	Logger       *logger.Logger `json:"-"`
}

func UploadInfoFrom(log *logger.Logger, storageURL *url.URL, path, filename, mimetype string, size int64) (*UploadInfo, error) {
	var err error
	info := &UploadInfo{
		MimeType: mimetype,
		Size:     size,
		Logger:   log.Child("uploadinfo", "create", "filename", filename),
	}

	info.ContentURL, err = storageURL.Parse(filename)
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasPrefix(mimetype, "image"):
		// TODO: If the file is an image, calculate a thumbnail
		thumbnail, err := info.getThumbnail(path)
		if err != nil {
			info.Logger.Warnf("Failed to create a thumbnail, we will use a default icon, Error: %s", err)
			info.ThumbnailURL, _ = url.Parse("https://cdn2.iconfinder.com/data/icons/freecns-cumulus/16/519587-084_Photo-64.png")
		} else {
			info.ThumbnailURL, _ = storageURL.Parse(thumbnail)
		}
	case strings.HasPrefix(mimetype, "audio"):
		info.ThumbnailURL, _ = url.Parse("https://cdn1.iconfinder.com/data/icons/ios-11-glyphs/30/circled_play-64.png")
		// TODO: If the file is an audio, calculate its duration in seconds
	case strings.HasPrefix(mimetype, "video"):
		info.ThumbnailURL, _ = url.Parse("https://cdn2.iconfinder.com/data/icons/flat-ui-icons-24-px/24/video-24-64.png")
		// TODO: If the file is a video?!? thumbnail (with an icon in the middle?!?), duration?
	case mimetype == "application/pdf":
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
		C *core.URL     `json:"contentUrl"`
		T *core.URL     `json:"thumbnailUrl,omitempty"`
		D core.Duration `json:"duration,omitempty"`
	}{
		C:         (*core.URL)(info.ContentURL),
		T:         (*core.URL)(info.ThumbnailURL),
		D:         (core.Duration)(info.Duration),
		surrogate: surrogate(info),
	})
	return data, errors.JSONMarshalError.Wrap(err)
}