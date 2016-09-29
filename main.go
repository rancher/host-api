package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/rancher/host-api/config"
	"github.com/rancher/host-api/console"
	"github.com/rancher/host-api/dockersocketproxy"
	"github.com/rancher/host-api/events"
	"github.com/rancher/host-api/exec"
	"github.com/rancher/host-api/healthcheck"
	"github.com/rancher/host-api/logs"
	"github.com/rancher/host-api/proxy"
	"github.com/rancher/host-api/stats"
	"github.com/rancher/host-api/util"

	"github.com/golang/glog"

	rclient "github.com/rancher/go-rancher/client"
	"github.com/rancher/host-api/cadvisor"
	"github.com/rancher/websocket-proxy/backend"
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
	tokenResponse, err := getConnectionToken(0, tokenRequest, rancherClient)
	if err != nil {
		logrus.Fatal(err)
	} else if tokenResponse == nil {
		// nil error and blank token means the proxy is turned off. Just block forever so main function doesn't exit
		var block chan bool
		<-block
	}

	if err := cadvisor.StartUp(); err != nil {
		logrus.Error(err)
	}

	handlers := make(map[string]backend.Handler)
	handlers["/v1/logs/"] = &logs.LogsHandler{}
	handlers["/v2-beta/logs/"] = &logs.LogsHandler{}
	handlers["/v1/stats/"] = &stats.StatsHandler{}
	handlers["/v2-beta/stats/"] = &stats.StatsHandler{}
	handlers["/v1/hoststats/"] = &stats.HostStatsHandler{}
	handlers["/v2-beta/hoststats/"] = &stats.HostStatsHandler{}
	handlers["/v1/containerstats/"] = &stats.ContainerStatsHandler{}
	handlers["/v2-beta/containerstats/"] = &stats.ContainerStatsHandler{}
	handlers["/v1/exec/"] = &exec.ExecHandler{}
	handlers["/v2-beta/exec/"] = &exec.ExecHandler{}
	handlers["/v1/console/"] = &console.Handler{}
	handlers["/v2-beta/console/"] = &console.Handler{}
	handlers["/v1/dockersocket/"] = &dockersocketproxy.Handler{}
	handlers["/v1/container-proxy/"] = &proxy.Handler{}
	handlers["/v2-beta/dockersocket/"] = &dockersocketproxy.Handler{}
	handlers["/v2-beta/container-proxy/"] = &proxy.Handler{}
	backend.ConnectToProxy(tokenResponse.Url+"?token="+tokenResponse.Token, handlers)
}

const maxWaitOnHostTries = 20

func getConnectionToken(try int, tokenReq *rclient.HostApiProxyToken, rancherClient *rclient.RancherClient) (*rclient.HostApiProxyToken, error) {
	if try >= maxWaitOnHostTries {
		return nil, fmt.Errorf("Reached max retry attempts for getting token.")
	}

	tokenResponse, err := rancherClient.HostApiProxyToken.Create(tokenReq)
	if err != nil {
		if apiError, ok := err.(*rclient.ApiError); ok {
			if apiError.StatusCode == 422 {
				parsed := &ParsedError{}
				if uErr := json.Unmarshal([]byte(apiError.Body), &parsed); uErr == nil {
					if strings.EqualFold(parsed.Code, "InvalidReference") && strings.EqualFold(parsed.FieldName, "reportedUuid") {
						logrus.WithField("reportedUuid", config.Config.HostUuid).WithField("Attempt", try).Infof("Host not registered yet. Sleeping 1 second and trying again.")
						time.Sleep(time.Second)
						try += 1
						return getConnectionToken(try, tokenReq, rancherClient) // Recursion!
					}
				} else {
					return nil, uErr
				}
			} else if apiError.StatusCode == 501 {
				logrus.Infof("Host-api proxy disabled. Will not connect.")
				return nil, nil
			}
			return nil, err
		}
	}
	return tokenResponse, nil
}

type ParsedError struct {
	Code      string
	FieldName string
}
