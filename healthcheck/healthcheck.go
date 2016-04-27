package healthcheck

import (
	"fmt"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	set "github.com/deckarep/golang-set"
	lru "github.com/hashicorp/golang-lru"
	"github.com/rancherio/go-rancher/client"
	"github.com/rancherio/host-api/pkg/haproxy"
	"github.com/rancherio/host-api/util"
)

var PREFIX = "cattle-"
var SERVER_NAME = "svname"
var STATUS = "status"

func Poll() error {
	client, err := util.GetRancherClient()
	if err != nil {
		return err
	}
	if client == nil {
		return fmt.Errorf("Can not create RancherClient, No credentials found")
	}

	m := &Monitor{
		client:         client,
		reportedStatus: map[string]string{},
	}

	c, _ := lru.New(500)
	go m.readStats(c)

	for {
		// Sleep here as well, so you don't end up reading from cache non stop
		time.Sleep(2 * time.Second)
		for _, key := range c.Keys() {
			var stat haproxy.Stat
			s, _ := c.Get(key)
			if s == nil {
				continue
			}
			if v, ok := s.(haproxy.Stat); ok {
				stat = v
				m.processStat(stat)
			}
		}
	}

	return nil
}

type Monitor struct {
	client         *client.RancherClient
	reportedStatus map[string]string
}

func (m *Monitor) readStats(c *lru.Cache) {
	count := 0

	h := &haproxy.Monitor{
		SocketPath: haproxy.HAPROXY_SOCK,
	}

	for {
		// Sleep up front.  This way if this program gets restarted really fast we don't spam cattle
		time.Sleep(2 * time.Second)
		stats, err := h.Stats()
		currentCount := 0
		if err != nil {
			logrus.Errorf("Failed to read stats: %v", err)
			continue
		}

		servers := set.NewSet()
		for _, stat := range stats {
			if strings.HasPrefix(stat[SERVER_NAME], PREFIX) {
				currentCount++
				c.Add(stat[SERVER_NAME], stat)
				servers.Add(stat[SERVER_NAME])
			}
		}

		//cleanup obsolete cash entries
		for _, key := range c.Keys() {
			if s, ok := key.(string); ok {
				if !servers.Contains(s) {
					c.Remove(key)
					logrus.Debugf("Removing obsolete entry from cache %v", s)
				}
			}
		}

		if currentCount != count {
			count = currentCount
			logrus.Infof("Monitoring %d backends", count)
		}
	}
}

func (m *Monitor) processStat(stat haproxy.Stat) {
	serverName := strings.TrimPrefix(stat[SERVER_NAME], PREFIX)
	currentStatus := stat[STATUS]

	previousStatus := m.reportedStatus[serverName]
	if strings.HasPrefix(currentStatus, "UP ") {
		// do nothing on partial UP
		return
	}

	if currentStatus == "UP" && previousStatus != "UP" && previousStatus != "INIT" {
		currentStatus = "INIT"
	}

	if previousStatus != currentStatus {
		err := m.reportStatus(serverName, currentStatus)
		if err != nil {
			logrus.Errorf("Failed to report status %s=%s: %v", serverName, currentStatus, err)
		} else {
			m.reportedStatus[serverName] = currentStatus
		}
	}
}

func (m *Monitor) reportStatus(serverName, currentStatus string) error {
	_, err := m.client.ServiceEvent.Create(&client.ServiceEvent{
		HealthcheckUuid:   serverName,
		ReportedHealth:    currentStatus,
		ExternalTimestamp: time.Now().Unix(),
	})

	if err != nil {
		return err
	}

	logrus.Infof("%s=%s", serverName, currentStatus)
	return nil
}
