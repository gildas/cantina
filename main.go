package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gildas/wess"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	// Analyzing the command line arguments
	var (
		port           = flag.Int("port", core.GetEnvAsInt("PORT", 80), "the TCP port for which the server listens to")
		probePort      = flag.Int("probeport", core.GetEnvAsInt("PROBE_PORT", 0), "Start a Health web server for Kubernetes if > 0")
		storageRoot    = flag.String("storage-root", core.GetEnvAsString("STORAGE_ROOT", "/var/storage"), "the folder where all the files are stored")
		storageURLX    = flag.String("storage-url", core.GetEnvAsString("STORAGE_URL", ""), "the Storage URL for external access")
		corsOrigins    = flag.String("cors-origins", "*", "the comma-separated list of origins that are allowed to post (CORS)")
		appendAPI      = flag.Bool("append-api-url", core.GetEnvAsBool("STORAGE_APPEND_API_URL", true), "if true, appends \"/api/v1/files\" to the storage URL")
		purgeFrequency = flag.Duration("purge-frequency", core.GetEnvAsDuration("PURGE_FREQUENCY", 1*time.Minute), "the frequency the files are purged. Default: 1 minute")
		purgeAfter     = flag.Duration("purge-after", core.GetEnvAsDuration("PURGE_AFTER", 0*time.Second), "the duration after which files are purged. Default: never")
		version        = flag.Bool("version", false, "prints the current version and exits")
		wait           = flag.Duration("graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish")
	)
	flag.Parse()

	if *version {
		fmt.Printf("%s v%s (%s)\n", APP, Version(), runtime.GOARCH)
		os.Exit(0)
	}

	// Initializing the Logger
	log := logger.Create(APP)
	mainctx := log.ToContext(context.Background())
	defer log.Flush()
	log.Infof("%s", strings.Repeat("-", 80))
	log.Infof("Starting %s v%s (%s)", APP, Version(), runtime.GOARCH)
	log.Infof("Log Destination: %s", log)
	log.Infof("Webserver Port=%d, Health Port=%d", *port, *probePort)
	log.Infof("Storage location: %s", *storageRoot)
	if *purgeAfter == 0 {
		log.Infof("Default purge: never")
	} else {
		log.Infof("Default purge: %s", *purgeAfter)
	}

	// Validating the storage URL
	storageURL, err := url.Parse(*storageURLX)
	if err != nil {
		log.Fatalf("Provided Storage URL (%s) is invalid", *storageURLX, err)
		log.Close()
		os.Exit(-1)
	}
	if *appendAPI {
		storageURL, _ = storageURL.Parse("api/v1/files/")
	}

	// Creating the folders
	if _, err := os.Stat(*storageRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(*storageRoot, os.ModePerm); err != nil {
			log.Fatalf("Failed to create the storage folder", err)
			log.Close()
			os.Exit(-1)
		}
	}
	metaRoot := filepath.Join(*storageRoot, ".meta")
	if _, err := os.Stat(metaRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(metaRoot, os.ModePerm); err != nil {
			log.Fatalf("Failed to create the meta-information folder", err)
			log.Close()
			os.Exit(-1)
		}
	}
	authRoot := filepath.Join(*storageRoot, ".auth")
	if _, err := os.Stat(authRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(authRoot, os.ModePerm); err != nil {
			log.Fatalf("Failed to create the authorization folder", err)
			log.Close()
			os.Exit(-1)
		}
	}

	// Create the Config object
	config := Config{
		MetaRoot:       metaRoot,
		PurgeAfter:     *purgeAfter,
		PurgeFrequency: *purgeFrequency,
		StorageRoot:    *storageRoot,
		StorageURL:     *storageURL,
	}

	// Starting the Purge Job
	var waitForJobs sync.WaitGroup
	_, stopPurge := StartPurge(config, &waitForJobs, log)

	// Initializing and starting the server
	server := wess.NewServer(wess.ServerOptions{
		Port:                 *port,
		ProbePort:            *probePort,
		ShutdownTimeout:      *wait,
		AllowedCORSOrigins:   strings.Split(*corsOrigins, ","),
		AllowedCORSHeaders:   []string{"Accept", "Accept-Encoding", "Authorization", "Connection", "Content-Length", "Content-Type", "Host", "User-Agent", "X-Request-Id", "X-Requested-With"},
		AllowedCORSMethods:   []string{http.MethodPost, http.MethodGet, http.MethodDelete},
		CORSAllowCredentials: true,
		Logger:               log,
	})
	log.Topic("cors").Infof("Allowed Origins: %v", strings.Split(*corsOrigins, ","))

	// Setting up web router
	authority := Authority{authRoot}
	apiRouter := server.SubRouter("/api/v1")
	apiRouter.Use(authority.Middleware(), config.HttpHandler())
	FilesRoutes(apiRouter)

	downloadRouter := server.SubRouter("/api/v1/files")
	downloadRouter.Methods(http.MethodGet).Handler(http.StripPrefix("/api/v1/files/", http.FileServer(StorageFileSystem{http.Dir(*storageRoot)})))

	HealthRoutes(server.SubRouter("/healthz"))

	// Starting the web server
	shutdown, _, err := server.Start(mainctx)
	if err != nil {
		log.Fatalf("Failed to start the server", err)
		os.Exit(-1)
	}
	atomic.StoreInt32(&HealthHTTP, 1)

	// Waiting for the server to shutdown (SIGINT (^C) and SIGTERM (docker, heroku))
	err = <-shutdown
	if err != nil {
		log.Fatalf("Failed to shutdown the server", err)
		os.Exit(-1)
	}
	// Stopping the Purge Job
	close(stopPurge)

	// Wait for all jobs to finish
	waitForJobs.Wait()
	log.Infof("All job have stopped")
	os.Exit(0)
}
