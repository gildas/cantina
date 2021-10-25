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
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// Log is the application Logger
var Log *logger.Logger

// WebServer is the application Web Server
var WebServer *http.Server

func main() {
	_ = godotenv.Load()
	// Analyzing the command line arguments
	var (
		port        = flag.Int("port", core.GetEnvAsInt("PORT", 80), "the TCP port for which the server listens to")
		probePort   = flag.Int("probeport", core.GetEnvAsInt("PROBE_PORT", 0), "Start a Health web server for Kubernetes if > 0")
		storageRoot = flag.String("storage-root", core.GetEnvAsString("STORAGE_ROOT", "/var/storage"), "the folder where all the files are stored")
		storageURLX = flag.String("storage-url", core.GetEnvAsString("STORAGE_URL", ""), "the Storage URL for external access")
		corsOrigins = flag.String("cors-origins", "*", "the comma-separated list of origins that are allowed to post (CORS)")
		appendAPI   = flag.Bool("append-api-url", core.GetEnvAsBool("STORAGE_APPEND_API_URL", true), "if true, appends \"/api/v1/files\" to the storage URL")
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
	Log.Infof("Starting %s v. %s", APP, Version())
	Log.Infof("Log Destination: %s", Log)
	Log.Infof("Webserver Port=%d, Health Port=%d", *port, *probePort)
	Log.Infof("Storage location: %s", *storageRoot)

	// Validating the storage URL
	storageURL, err := url.Parse(*storageURLX)
	if err != nil {
		Log.Fatalf("Provided Storage URL (%s) is invalid", *storageURLX, err)
		Log.Close()
		os.Exit(-1)
	}
	if *appendAPI {
		storageURL, _ = storageURL.Parse("api/v1/files/")
	}

	// Creating the folders
	if _, err := os.Stat(*storageRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(*storageRoot, os.ModePerm); err != nil {
			Log.Fatalf("Failed to create the storage folder", err)
			Log.Close()
			os.Exit(-1)
		}
	}
	metaRoot := filepath.Join(*storageRoot, ".meta")
	if _, err := os.Stat(metaRoot); os.IsNotExist(err) {
		if err = os.MkdirAll(metaRoot, os.ModePerm); err != nil {
			Log.Fatalf("Failed to create the meta-information folder", err)
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
	authority := Authority{authRoot}

	// Create the Config object
	config := Config{
		MetaRoot:    metaRoot,
		StorageRoot: *storageRoot,
		StorageURL:  *storageURL,
	}

	// Setting up web router
	router := mux.NewRouter().StrictSlash(true)
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(Log.HttpHandler(), authority.HttpHandler(), config.HttpHandler())

	FilesRoutes(apiRouter)
	router.PathPrefix("/api/v1/files").Handler(http.StripPrefix("/api/v1/files/", http.FileServer(StorageFileSystem{http.Dir(*storageRoot)})))

	// Setting up CORS
	cors := []handlers.CORSOption{
		handlers.AllowedOrigins(strings.Split(*corsOrigins, ",")),
		handlers.AllowedMethods([]string{http.MethodPost, http.MethodGet, http.MethodOptions}),
	}
	Log.Topic("cors").Infof("Allowed Origins: %v", strings.Split(*corsOrigins, ","))

	// Setting up the Health server
	var healthServer *http.Server

	if *probePort > 0 {
		if *probePort != *port {
			// Assigning Health routes
			healthRouter := mux.NewRouter().StrictSlash(true).PathPrefix("/healthz").Subrouter()
			if len(core.GetEnvAsString("TRACE_PROBE", "")) > 0 {
				healthRouter.Use(Log.HttpHandler())
			}
			HealthRoutes(healthRouter)

			// Initializing the Health Server
			healthServer = &http.Server{
				Addr:         fmt.Sprintf("0.0.0.0:%d", *probePort),
				WriteTimeout: time.Second * 15,
				ReadTimeout:  time.Second * 15,
				IdleTimeout:  time.Second * 60,
				Handler:      healthRouter,
			}

			// Starting the health server
			go func() {
				log := Log.Child("healthserver", "run")

				log.Infof("Starting Health server on port %d", *probePort)
				if err := healthServer.ListenAndServe(); err != nil {
					if err.Error() != "http: Server closed" {
						log.Fatalf("Failed to start the Health server on port: %d", *probePort, err)
					}
				}
			}()
		} else {
			Log.Topic("healthserver").Scope("config").Infof("Health Server will run on the same port as the Web Server: (probe port: %d)", *probePort)
			healthRouter := router.PathPrefix("/healthz").Subrouter()
			if len(core.GetEnvAsString("TRACE_PROBE", "")) > 0 {
				healthRouter.Use(Log.HttpHandler())
			}
			HealthRoutes(healthRouter)
		}
	} else {
		Log.Topic("healthserver").Scope("config").Infof("Health Server will not run (probe port: %d)", *probePort)
	}

	// Initializing the web server
	WebServer = &http.Server{
		Addr:         fmt.Sprintf("0.0.0.0:%d", *port),
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      handlers.CORS(cors...)(router),
	}

	// Starting the web server
	go func() {
		log := Log.Child("webserver", "run")

		log.Infof("Starting WEB server on port %d", *port)
		log.Infof("Serving routes:")
		_ = router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			message := strings.Builder{}
			args := []interface{}{}

			if methods, err := route.GetMethods(); err == nil {
				message.WriteString("%s ")
				args = append(args, strings.Join(methods, ","))
			} else {
				return nil
			}
			if path, err := route.GetPathTemplate(); err == nil {
				message.WriteString("%s ")
				args = append(args, path)
			}
			if path, err := route.GetPathRegexp(); err == nil {
				message.WriteString("%s ")
				args = append(args, path)
			}
			log.Infof(message.String(), args...)
			return nil
		})
		atomic.StoreInt32(&HealthHTTP, 1)
		// TODO TLS
		if err := WebServer.ListenAndServe(); err != nil {
			atomic.StoreInt32(&HealthHTTP, 0)
			if err.Error() != "http: Server closed" {
				log.Fatalf("Failed to start the WEB server on port: %d", *port, err)
			}
		}
	}()

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
		if *probePort > 0 && *probePort != *port {
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
