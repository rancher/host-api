package stats

import (
	"bufio"
	"encoding/json"
	"io"
	"net/url"
	"time"

	log "github.com/Sirupsen/logrus"
	info "github.com/google/cadvisor/info/v1"

	"github.com/google/cadvisor/manager"
	"github.com/rancher/websocket-proxy/backend"
	"github.com/rancher/websocket-proxy/common"
)

type StatsHandler struct {
	CadvisorManager *manager.Manager
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

	//c, err := client.NewClient(config.Config.CAdvisorUrl)
	//if err != nil {
	//	log.WithFields(log.Fields{"error": err}).Error("Couldn't get CAdvisor client.")
	//	return
	//}

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

	for {
		machineInfo, err := (*(s.CadvisorManager)).GetMachineInfo()
		if err != nil {
			log.WithFields(log.Fields{"error": err}).Error("Error getting machine info.")
			return
		}

		memLimit := machineInfo.MemoryCapacity

		containerInfo, err := getContainerStats(count, id)
		if err != nil {
			return
		}

		if err := writeStats(containerInfo, memLimit, writer); err != nil {
			return
		}

		time.Sleep(1 * time.Second)
		count = 1
	}
	return
}

func writeStats(info *info.ContainerInfo, memLimit uint64, writer io.Writer) error {
	for _, stat := range info.Stats {
		data, err := json.Marshal(stat)
		if err != nil {
			return err
		}

		_, err = writer.Write(data)
		if err != nil {
			return err
		}

		_, err = writer.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}
	return nil
}
