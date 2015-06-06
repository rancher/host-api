package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/google/cadvisor/client"
	"github.com/google/cadvisor/info"

	"github.com/rancherio/host-api/config"
	"github.com/rancherio/websocket-proxy/backend"
	"github.com/rancherio/websocket-proxy/common"
)

type StatsHandler struct {
}

func pathParts(path string) []string {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	return strings.Split(path, "/")
}

func (s *StatsHandler) Handle(key string, initialMessage string, incomingMessages <-chan string, response chan<- common.Message) {
	defer backend.SignalHandlerClosed(key, response)

	requestUrl, err := url.Parse(initialMessage)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "message": initialMessage}).Error("Couldn't parse url from message.")
		return
	}

	id := ""
	parts := pathParts(requestUrl.Path)
	if len(parts) == 3 {
		id = parts[2]
	}

	container, err := resolveContainer(id)
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
				log.Info("Incoming message channel closed. Exiting.")
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
			log.WithFields(log.Fields{"error": err}).Error("Error with the container log scanner.")
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

		info, err := c.ContainerInfo(container, &info.ContainerInfoRequest{
			NumStats: count,
		})
		if err != nil {
			return
		}

		err = writeStats(info, memLimit, writer)
		if err != nil {
			return
		}

		time.Sleep(1 * time.Second)
		count = 1
	}

	return
}

func writeStats(info *info.ContainerInfo, memLimit int64, writer io.Writer) error {
	for _, stat := range info.Stats {
		stat.Memory.Limit = uint64(memLimit)
		data, err := json.Marshal(stat)
		if err != nil {
			return err
		}

		writer.Write(data)
	}

	return nil
}

func resolveContainer(id string) (string, error) {
	if id == "" {
		return "", nil
	}

	client, err := dockerClient.NewClient(config.Config.DockerUrl)
	if err != nil {
		return "", err
	}

	container, err := client.InspectContainer(id)
	if err != nil || container == nil {
		return "", err
	}

	if useSystemd() {
		return fmt.Sprintf("system.slice/docker-%s.scope", container.ID), nil
	} else {
		return fmt.Sprintf("docker/%s", container.ID), nil
	}
}

func useSystemd() bool {
	s, err := os.Stat("/run/systemd/system")
	if err != nil || !s.IsDir() {
		return false
	}

	return true
}
