package main

import (
	"flag"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/Sirupsen/logrus"
	"github.com/rancherio/host-api/config"
	"github.com/rancherio/host-api/events"
	"github.com/rancherio/host-api/exec"
	"github.com/rancherio/host-api/healthcheck"
	"github.com/rancherio/host-api/logs"
	"github.com/rancherio/host-api/stats"
	"github.com/rancherio/host-api/util"

	"github.com/golang/glog"

	rclient "github.com/rancherio/go-rancher/client"
	"github.com/rancherio/websocket-proxy/backend"
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

	rancherClient, err := util.GetRancherClient()
	if err != nil {
		logrus.Fatal(err)
	}

	tokenRequest := &rclient.HostApiProxyToken{
		ReportedUuid: config.Config.HostUuid,
	}
	tokenResponse, err := rancherClient.HostApiProxyToken.Create(tokenRequest)
	if err != nil {
		logrus.Fatal(err)
	}

	handlers := make(map[string]backend.Handler)
	handlers["/v1/logs/"] = &logs.LogsHandler{}
	handlers["/v1/stats/"] = &stats.StatsHandler{}
	handlers["/v1/exec/"] = &exec.ExecHandler{}
	backend.ConnectToProxy(tokenResponse.Url+"?token="+tokenResponse.Token, config.Config.HostUuid, handlers)
}
