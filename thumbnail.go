package main

import (
	"strings"
	"time"
)

type Thumbnail struct {
	MimeType string
	Filename string
	Duration time.Duration
}

func ThumbnailFrom(mimetype string) *Thumbnail {
		// TODO: If the file is an image, calculate a thumbnail
		// TODO: Otherwise get an "icon" of the mimetype
		// TODO: If the file is an audio, calculate its duration in seconds
		// TODO: If the file is a video?!? thumbnail (with an icon in the middle?!?), duration?
	switch {
	case strings.HasPrefix(mimetype, "image"):
	case strings.HasPrefix(mimetype, "audio"):
	case strings.HasPrefix(mimetype, "video"):
	case mimetype == "application/pdf":
	default:
	}
} 