package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/rancherio/host-api/app/common"
	"github.com/rancherio/host-api/auth"
	"github.com/rancherio/host-api/config"
	"github.com/rancherio/host-api/events"
	"github.com/rancherio/host-api/healthcheck"
	"github.com/rancherio/host-api/logs"
	"github.com/rancherio/host-api/stats"

	"github.com/golang/glog"
	"github.com/gorilla/mux"
)

func main() {
	err := config.Parse()
	if err != nil {
		logrus.Fatal(err)
	}

	flag.Parse()
	defer glog.Flush()

	if config.Config.PidFile != "" {
		logrus.Infof("Writing pid %d to %s", os.Getpid(), config.Config.PidFile)
		if err := ioutil.WriteFile(config.Config.PidFile, []byte(strconv.Itoa(os.Getpid())), 0644); err != nil {
			logrus.Fatalf("Failed to write pid file %s: %v", config.Config.PidFile, err)
		}
	}

	if config.Config.LogFile != "" {
		if output, err := os.OpenFile(config.Config.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666); err != nil {
			logrus.Fatalf("Failed to log to file %s: %v", config.Config.LogFile, err)
		} else {
			logrus.SetOutput(output)
		}
	}

	if config.Config.HaProxyMonitor {
		err := healthcheck.Poll()
		if err != nil {
			logrus.Fatal(err)
		}
		os.Exit(0)
	}

	processor := events.NewDockerEventsProcessor(config.Config.EventsPoolSize)
	err = processor.Process()
	if err != nil {
		logrus.Fatal(err)
	}

	router := mux.NewRouter()
	http.Handle("/", auth.AuthHttpInterceptor(router))

	router.Handle("/v1/stats", common.ErrorHandler(stats.GetStats)).Methods("GET")
	router.Handle("/v1/stats/{id}", common.ErrorHandler(stats.GetStats)).Methods("GET")
	router.Handle("/v1/logs/", common.ErrorHandler(logs.GetLogs)).Methods("GET")

	var listen = fmt.Sprintf("%s:%d", config.Config.Ip, config.Config.Port)
	err = http.ListenAndServe(listen, nil)

	if err != nil {
		logrus.Fatal(err)
	}
}
