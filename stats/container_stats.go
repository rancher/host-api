package stats

import (
	"bufio"
	"io"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/rancher/host-api/config"
	"github.com/rancher/websocket-proxy/backend"
	"github.com/rancher/websocket-proxy/common"
	"golang.org/x/net/context"
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

	dclient, err := client.NewEnvClient()
	if err != nil {
		log.WithFields(log.Fields{"error": err}).Error("Couldn't get docker client.")
		return
	}
	dclient.UpdateClientVersion("1.22")

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

	count := 1
	memLimit, err := getMemCapcity()
	if err != nil {
		log.WithFields(log.Fields{"error": err, "id": id}).Error("Error getting memory capacity.")
		return
	}

	// get single container stats
	if id != "" {
		statsReader, err := dclient.ContainerStats(context.Background(), id, true)
		defer statsReader.Close()
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Can not get stats reader from docker")
			return
		}
		bufioReader := bufio.NewReader(statsReader)
		for {
			infos := []containerInfo{}
			cInfo, err := getContainerStats(bufioReader, count, id)

			if err != nil {
				log.WithFields(log.Fields{"error": err, "id": id}).Error("Error getting container info.")
				return
			}
			infos = append(infos, cInfo)
			for i := range infos {
				if len(infos[i].Stats) > 0 {
					infos[i].Stats[0].Timestamp = time.Now()
				}
			}

			err = writeAggregatedStats(id, containerIds, "container", infos, uint64(memLimit), writer)
			if err != nil {
				return
			}

			time.Sleep(1 * time.Second)
			count = 1
		}
	} else {
		contList, err := dclient.ContainerList(context.Background(), types.ContainerListOptions{})
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Can not list containers")
			return
		}
		IDList := []string{}
		bufioReaders := []*bufio.Reader{}
		for _, cont := range contList {
			if _, ok := containerIds[cont.ID]; ok {
				statsReader, err := dclient.ContainerStats(context.Background(), cont.ID, true)
				defer statsReader.Close()
				if err != nil {
					log.WithFields(log.Fields{"error": err}).Error("Can not get stats reader from docker")
					return
				}
				bufioReader := bufio.NewReader(statsReader)
				bufioReaders = append(bufioReaders, bufioReader)
				IDList = append(IDList, cont.ID)
			}
		}
		for {
			infos := []containerInfo{}
			allInfos, err := getAllDockerContainers(bufioReaders, count, IDList)
			if err != nil {
				log.WithFields(log.Fields{"error": err}).Error("Error getting all container info.")
				return
			}
			infos = append(infos, allInfos...)
			for i := range infos {
				if len(infos[i].Stats) > 0 {
					infos[i].Stats[0].Timestamp = time.Now()
				}
			}
			err = writeAggregatedStats(id, containerIds, "container", infos, uint64(memLimit), writer)
			if err != nil {
				return
			}

			time.Sleep(1 * time.Second)
		}
	}

	return
}
