package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gorilla/mux"
)

// Log is the application Logger
var Log *logger.Logger

// WebServer is the application Web Server
var WebServer *http.Server

func main() {
	// Analyzing the command line arguments
	var (
		port        = flag.Int("port", core.GetEnvAsInt("PORT", 80), "the TCP port for which the server listens to")
		probePort   = flag.Int("probeport", core.GetEnvAsInt("PROBE_PORT", 0), "Start a Health web server for Kubernetes if > 0")
		storageRoot = flag.String("storage-root", core.GetEnvAsString("STORAGE_ROOT", "/var/storage"), "the folder where all the files are stored")
		storageURLX = flag.String("storage-url", core.GetEnvAsString("STORAGE_URL", ""), "the Storage URL for external access")
		version     = flag.Bool("version", false, "prints the current version and exits")
		wait        = flag.Duration("graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish")
	)
	flag.Parse()

	if *version {
		fmt.Printf("%s version %s\n", APP, VERSION)
		os.Exit(0)
	}

	// Initializing the Logger
	Log = logger.Create(APP)
	defer Log.Flush()
	Log.Infof(strings.Repeat("-", 80))
	Log.Infof("Starting %s v. %s", APP, VERSION)
	Log.Infof("Log Destination: %s", Log)
	Log.Infof("Webserver Port=%d, Health Port=%d", *port, *probePort)

	// Validating the storage URL
	storageURL, err := url.Parse(*storageURLX)
	if err != nil {
		Log.Fatalf("Provided Storage URL (%s) is invalid", *storageURLX, err)
		Log.Close()
		os.Exit(-1)
	}
	storageURL, _ = storageURL.Parse("api/v1/files/")

	// Creating the storage folder
	if _, err := os.Stat(*storageRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(*storageRoot, os.ModePerm); err != nil {
			Log.Fatalf("Failed to create the storage folder", err)
			Log.Close()
			os.Exit(-1)
		}
	}
	authRoot := filepath.Join(*storageRoot, ".auth")
	if _, err := os.Stat(authRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(authRoot, os.ModePerm); err != nil {
			Log.Fatalf("Failed to create the authorization folder", err)
			Log.Close()
			os.Exit(-1)
		}
	}

	// Setting up web routes
	router := mux.NewRouter().StrictSlash(true)
	FilesRoutes(router, "/api/v1", *storageRoot, storageURL, Log)
	router.PathPrefix("/api/v1/files").Handler(http.StripPrefix("/api/v1/files/", http.FileServer(StorageFileSystem{http.Dir(*storageRoot)})))
	if *probePort == *port {
		HealthRoutes(router, "/healthz", Log)
	}

	// Initializing the web server
	WebServer = &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", *port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      router,
	}

	// Starting the web server
	go func() {
		log := Log.Child("webserver", "run")

		log.Infof("Starting WEB server on port %d", *port)
		atomic.StoreInt32(&HealthHTTP, 1)
		// TODO TLS
		if err := WebServer.ListenAndServe(); err != nil {
			atomic.StoreInt32(&HealthHTTP, 0)
			if err.Error() != "http: Server closed" {
				log.Fatalf("Failed to start the WEB server on port: %d", *port, err)
			}
		}
	}()

	// Setting up the Health server
	var healthServer *http.Server

	if *probePort > 0  && *probePort != *port {
		// Assigning Health routes
		healthRouter := mux.NewRouter().StrictSlash(true)
		HealthRoutes(healthRouter, "/healthz", Log)

		// Initializing the Health Server
		healthServer = &http.Server{
			Addr:         fmt.Sprintf("0.0.0.0:%d", *probePort),
			WriteTimeout: time.Second * 15,
			ReadTimeout:  time.Second * 15,
			IdleTimeout:  time.Second * 60,
			Handler:      healthRouter,
		}

		// Starting the server
		go func() {
			log := Log.Child("healthserver", "run")

			log.Infof("Starting Health server on port %d", *probePort)
			if err := healthServer.ListenAndServe(); err != nil {
				if err.Error() != "http: Server closed" {
					log.Fatalf("Failed to start the Health server on port: %d", *probePort, err)
				}
			}
		}()
	} else if *probePort == *port {
		Log.Topic("healthserver").Scope("config").Infof("Health Server will run on the same port as the Web Server: (probe port: %d)", *probePort)
	} else {
		Log.Topic("healthserver").Scope("config").Infof("Health Server will not run (probe port: %d)", *probePort)
	}

	// Accepting shutdowns from SIGINT (^C) and SIGTERM (docker, heroku)
	interruptChannel := make(chan os.Signal, 1)
	exitChannel := make(chan struct{})
	signal.Notify(interruptChannel, os.Interrupt, syscall.SIGTERM)

	// Waiting to clean stuff up when exiting
	go func() {
		sig := <-interruptChannel // Block until we have to stop

		context, cancel := context.WithTimeout(context.Background(), *wait)
		defer cancel()

		Log.Infof("Application is stopping (%+v)", sig)

		// Stopping the Health server
		if *probePort > 0 {
			Log.Debugf("Health server is shutting down")
			healthServer.SetKeepAlivesEnabled(false)
			_ = healthServer.Shutdown(context)
			Log.Infof("Health server is stopped")
		}

		// Stopping the WEB server
		if *port != 0 {
			Log.Debugf("WEB server is shutting down")
			atomic.StoreInt32(&HealthHTTP, 0)
			WebServer.SetKeepAlivesEnabled(false)
			_ = WebServer.Shutdown(context)
			Log.Infof("WEB server is stopped")
		}

		Log.Close()
		close(exitChannel)
	}()

	<-exitChannel
	os.Exit(0)
}