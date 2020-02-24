package main

import (
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"github.com/gildas/go-core"
	"github.com/disintegration/imaging"
	"github.com/gildas/go-errors"
)

type UploadInfo struct {
	ContentURL   *url.URL      `json:"-"`
	ThumbnailURL *url.URL      `json:"-"`
	Duration     time.Duration `json:"-"`
	MimeType     string        `json:"mimeType"`
	Size         int64         `json:"size"`
}

func UploadInfoFrom(storageURL *url.URL, path, filename, mimetype string, size int64) (*UploadInfo, error) {
	var err error
	info := &UploadInfo{
		MimeType: mimetype,
		Size:     size,
	}

	info.ContentURL, err = storageURL.Parse(filename)
	if err != nil {
		return nil, err
	}

	switch {
	case strings.HasPrefix(mimetype, "image"):
		// TODO: If the file is an image, calculate a thumbnail
		thumbnail, err := getThumbnail(path)
		if err != nil {
			return nil ,err
		}
		info.ThumbnailURL, _ = storageURL.Parse(thumbnail)
		// info.ThumbnailURL, _ = url.Parse("https://cdn2.iconfinder.com/data/icons/freecns-cumulus/16/519587-084_Photo-64.png")
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

func (info UploadInfo) MarshalJSON() ([]byte, error) {
	type surrogate UploadInfo
	data, err := json.Marshal(struct {
		surrogate
		C *core.URL     `json:"contentUrl"`
		T *core.URL     `json:"thumnailUrl,omitempty"`
		D core.Duration `json:"duration,omitempty"`
	}{
		C:         (*core.URL)(info.ContentURL),
		T:         (*core.URL)(info.ThumbnailURL),
		D:         (core.Duration)(info.Duration),
		surrogate: surrogate(info),
	})
	return data, errors.JSONMarshalError.Wrap(err)
}

func getThumbnail(path string) (string, error) {
	original, err := imaging.Open(path)
	if err != nil {
		return "", nil
	}

	thumbnail := imaging.Thumbnail(original, 64, 64, imaging.CatmullRom)

	/*
	out, err := os.Create("thumbnail.png") //basename + filename-no-ext + -thumbnail + ext
	defer out.Close()
	//jpeg.Encode(out, thumbnail, &jpeg.Options{jpeg.DefaultQuality})
	png.Encode(out, thumbnail)
	*/

	err = imaging.Save(thumbnail, "thumbnail.png")
	if err != nil {
		return "", nil
	}
	return "thumbnail.png", nil
}