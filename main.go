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
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gildas/go-core"
	"github.com/gildas/go-logger"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

// WebServer is the application Web Server
var WebServer *http.Server

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
	authority := Authority{authRoot}

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

	// Setting up web router
	router := mux.NewRouter().StrictSlash(true)
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	apiRouter.Use(log.HttpHandler(), authority.HttpHandler(), config.HttpHandler())

	FilesRoutes(apiRouter)
	router.PathPrefix("/api/v1/files").Handler(http.StripPrefix("/api/v1/files/", http.FileServer(StorageFileSystem{http.Dir(*storageRoot)})))

	// Setting up CORS
	cors := []handlers.CORSOption{
		handlers.AllowedOrigins(strings.Split(*corsOrigins, ",")),
		handlers.AllowedMethods([]string{http.MethodPost, http.MethodGet, http.MethodOptions}),
	}
	log.Topic("cors").Infof("Allowed Origins: %v", strings.Split(*corsOrigins, ","))

	// Setting up the Health server
	var healthServer *http.Server

	if *probePort > 0 {
		if *probePort != *port {
			// Assigning Health routes
			healthRouter := mux.NewRouter().StrictSlash(true).PathPrefix("/healthz").Subrouter()
			if len(core.GetEnvAsString("TRACE_PROBE", "")) > 0 {
				healthRouter.Use(log.HttpHandler())
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
				log := log.Child("healthserver", "run")

				log.Infof("Starting Health server on port %d", *probePort)
				if err := healthServer.ListenAndServe(); err != nil {
					if err.Error() != "http: Server closed" {
						log.Fatalf("Failed to start the Health server on port: %d", *probePort, err)
					}
				}
			}()
		} else {
			log.Topic("healthserver").Scope("config").Infof("Health Server will run on the same port as the Web Server: (probe port: %d)", *probePort)
			healthRouter := router.PathPrefix("/healthz").Subrouter()
			if len(core.GetEnvAsString("TRACE_PROBE", "")) > 0 {
				healthRouter.Use(log.HttpHandler())
			}
			HealthRoutes(healthRouter)
		}
	} else {
		log.Topic("healthserver").Scope("config").Infof("Health Server will not run (probe port: %d)", *probePort)
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
		log := log.Child("webserver", "run")

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

		log.Infof("Application is stopping (%+v)", sig)

		// Stopping the Purge Job
		close(stopPurge)

		// Wait for all jobs to finish
		waitForJobs.Wait()
		log.Infof("All job have stopped")

		// Stopping the Health server
		if *probePort > 0 && *probePort != *port {
			log.Debugf("Health server is shutting down")
			healthServer.SetKeepAlivesEnabled(false)
			_ = healthServer.Shutdown(context)
			log.Infof("Health server is stopped")
		}

		// Stopping the WEB server
		if *port != 0 {
			log.Debugf("WEB server is shutting down")
			atomic.StoreInt32(&HealthHTTP, 0)
			WebServer.SetKeepAlivesEnabled(false)
			_ = WebServer.Shutdown(context)
			log.Infof("WEB server is stopped")
		}

		log.Close()
		close(exitChannel)
	}()

	<-exitChannel
	os.Exit(0)
}
