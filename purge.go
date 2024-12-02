package main

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gildas/go-errors"
	"github.com/gildas/go-logger"
)

type Purge struct {
	config    Config
	waitgroup *sync.WaitGroup
	Logger    *logger.Logger
}

// StartPurge starts a new Purge Job
func StartPurge(config Config, waitgroup *sync.WaitGroup, log *logger.Logger) (purge *Purge, stop chan struct{}) {
	stop = make(chan struct{})

	purge = &Purge{
		config:    config,
		waitgroup: waitgroup,
		Logger:    logger.CreateIfNil(log, "PURGE").Child("purge", "purge"),
	}

	waitgroup.Add(1)
	go purge.run(stop)

	return purge, stop
}

func (purge Purge) run(stop chan struct{}) {
	log := purge.Logger.Child(nil, "run")

	log.Infof("Running Purge Job every %s", purge.config.PurgeFrequency)
	timer := time.NewTicker(purge.config.PurgeFrequency)
	for {
		select {
		case <-stop:
			log.Infof("Stopping Purge Job")
			purge.waitgroup.Done()
			return
		case now := <-timer.C:
			log.Infof("Checking Metadata for files to purge (%s)", now)
			err := filepath.WalkDir(purge.config.MetaRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					log.Errorf("Failed to load %s", path, err)
					return errors.NotFound.With("path", path).(errors.Error).Wrap(err)
				}
				if entry.IsDir() {
					return nil
				}
				if filepath.Ext(path) != ".json" {
					return nil
				}
				log.Debugf("Loading %s", path)
				extension := filepath.Ext(path)
				basename := strings.TrimSuffix(filepath.Base(path), extension)
				context := log.Record("filename", basename).ToContext(context.Background())
				metadata := FindMetaInformation(context, purge.config, basename)
				if metadata.DeleteAt != nil {
					log.Debugf("File %s, should purge in %s on %s", metadata.Filename, metadata.DeleteAt.Sub(now), metadata.DeleteAt)
					if now.UTC().After(*metadata.DeleteAt) {
						if err = metadata.DeleteContent(context); err != nil {
							log.Errorf("Failed to delete content for %s", metadata.Filename, err)
							return err
						}
						if err = metadata.Delete(context); err != nil {
							log.Errorf("Failed to delete metadata for %s", metadata.Filename, err)
							return err
						}
						log.Infof("Deleted %s", metadata.Filename)
					}
				} else {
					log.Debugf("File %s is not marked for deletion", metadata.Filename)
				}
				return nil
			})
			if err != nil {
				log.Errorf("Failed to scan path %s", purge.config.MetaRoot, err)
			}
		}
	}
}
