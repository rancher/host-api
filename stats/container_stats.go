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

type ContainerStatsHandler struct {
}

func (s *ContainerStatsHandler) Handle(key string, initialMessage string, incomingMessages <-chan string, response chan<- common.Message) {
	defer backend.SignalHandlerClosed(key, response)

	requestUrl, err := url.Parse(initialMessage)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "message": initialMessage}).Error("Couldn't parse url from message.")
		return
	}

	tokenString := requestUrl.Query().Get("token")

	containerIds := map[string]string{}

	token, err := parseRequestToken(tokenString, config.Config.ParsedPublicKey)
	if err == nil {
		containerIdsInterface, found := token.Claims["containerIds"]
		if found {
			containerIdsVal, ok := containerIdsInterface.(map[string]interface{})
			if ok {
				for key, val := range containerIdsVal {
					if containerIdsValString, ok := val.(string); ok {
						containerIds[key] = containerIdsValString
					}
				}
			}
		}
	}

	id := ""
	parts := pathParts(requestUrl.Path)
	if len(parts) == 3 {
		id = parts[2]
	}

	if err != nil {
		log.WithFields(log.Fields{"id": id, "error": err}).Error("Couldn't find container for id.")
		return
	}

	c, err := client.NewClient(config.Config.CAdvisorUrl)
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Couldn't get CAdvisor client.")
		return
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

		if id != "" {
			cInfo, err := getContainerStats(c, count, id)
			if err != nil {
				log.WithFields(log.Fields{"error": err, "id": id}).Error("Error getting container info.")
				return
			}
			infos = append(infos, *cInfo)
		} else {
			cInfos, err := c.AllDockerContainers(&info.ContainerInfoRequest{
				NumStats: count,
			})
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Error("Error getting all container info.")
				return
			}
			infos = append(infos, cInfos...)
		}

		if count == 1 {
			for i := range infos {
				if len(infos[i].Stats) > 0 {
					infos[i].Stats[0].Timestamp = time.Now()
				}
			}
		}

		err = writeAggregatedStats(id, containerIds, "container", infos, uint64(memLimit), writer)
		if err != nil {
			return
		}

		time.Sleep(1 * time.Second)
		count = 1
	}

	return
}
