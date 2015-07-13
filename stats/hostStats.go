package stats

import (
	"bufio"
	"io"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/google/cadvisor/client"
	info "github.com/google/cadvisor/info/v1"

	"github.com/rancherio/host-api/config"
	"github.com/rancherio/websocket-proxy/backend"
	"github.com/rancherio/websocket-proxy/common"
)

type HostStatsHandler struct {
}

func (s *HostStatsHandler) Handle(key string, initialMessage string, incomingMessages <-chan string, response chan<- common.Message) {
	defer backend.SignalHandlerClosed(key, response)

	c, err := client.NewClient(config.Config.CAdvisorUrl)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Couldn't get CAdvisor client.")
		return
	}

	requestUrl, err := url.Parse(initialMessage)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "message": initialMessage}).Error("Couldn't parse url from message.")
		return
	}

	tokenString := requestUrl.Query().Get("token")

	resourceId := ""

	token, err := parseRequestToken(tokenString, config.Config.ParsedPublicKey)
	if err == nil {
		resourceIdInterface, found := token.Claims["resourceId"]
		if found {
			resourceIdVal, ok := resourceIdInterface.(string)
			if ok {
				resourceId = resourceIdVal
			}
		}
	}

	reader, writer := io.Pipe()

	go func(w *io.PipeWriter) {
		for {
			_, ok := <-incomingMessages
			if !ok {
				w.Close()
				return
			}
		}
	}(writer)

	go func(r *io.PipeReader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			message := common.Message{
				Key:  key,
				Type: common.Body,
				Body: text,
			}
			response <- message
		}
		if err := scanner.Err(); err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Error with the container stat scanner.")
		}
	}(reader)

	count := config.Config.NumStats

	for {
		machineInfo, err := c.MachineInfo()
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Error getting machine info.")
			return
		}

		memLimit := machineInfo.MemoryCapacity

		infos := []info.ContainerInfo{}

		cInfo, err := c.ContainerInfo("", &info.ContainerInfoRequest{
			NumStats: count,
		})
		if err != nil {
			return
		}

		infos = append(infos, *cInfo)

		err = writeAggregatedStats(resourceId, nil, "host", infos, uint64(memLimit), writer)
		if err != nil {
			return
		}

		time.Sleep(1 * time.Second)
		count = 1
	}

	return
}
