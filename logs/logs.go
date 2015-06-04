package logs

import (
	"bufio"
	"io"
	"net/url"
	"strconv"
	_ "time"

	log "github.com/Sirupsen/logrus"
	dockerClient "github.com/fsouza/go-dockerclient"

	"github.com/rancherio/websocket-proxy/backend"
	"github.com/rancherio/websocket-proxy/common"

	// "github.com/rancherio/host-api/app/common/connect"
	"github.com/rancherio/host-api/auth"
	"github.com/rancherio/host-api/events"
)

type LogsHandler struct {
}

func (l *LogsHandler) Handle(key string, initialMessage string, incomingMessages <-chan string, response chan<- common.Message) {
	defer backend.SignalHandlerClosed(key, response)

	log.Info("In the logger.")

	requestUrl, err := url.Parse(initialMessage)
	if err != nil {
		log.Error("Problem parsing url [%v] [%v]", requestUrl, err)
		return
	}
	tokenString := requestUrl.Query().Get("token")
	token, valid := auth.GetAndCheckToken(tokenString)
	if !valid {
		log.Error("Token ins not valid [%v]", token)
		return
	}

	logs := token.Claims["logs"].(map[string]interface{})
	container := logs["Container"].(string)
	follow, found := logs["Follow"].(bool)

	if !found {
		follow = true
	}

	tailTemp, found := logs["Lines"].(int)
	var tail string
	if found {
		tail = strconv.Itoa(int(tailTemp))
	} else {
		tail = "100"
	}

	client, err := events.NewDockerClient()
	if err != nil {
		log.Error("Problem getting docker client [%v]", err)
		return
	}

	//_, writer := io.Pipe()
	reader, writer := io.Pipe()

	containerRef, err := client.InspectContainer(container)
	if err != nil {
		return
	}

	logopts := dockerClient.LogsOptions{
		Container:  container,
		Follow:     follow,
		Stdout:     true,
		Stderr:     true,
		Timestamps: true,
		Tail:       tail,
	}
	if containerRef.Config.Tty {
		logopts.OutputStream = stdbothWriter{writer}
		logopts.RawTerminal = true
	} else {
		logopts.OutputStream = stdoutWriter{writer}
		logopts.ErrorStream = stderrorWriter{writer}
		logopts.RawTerminal = false
	}

	go func(r *io.PipeReader, w *io.PipeWriter) {
		for {
			_, ok := <-incomingMessages
			if !ok {
				log.Info("Incoming message channel closed. Exiting.")
				w.Close()
				return
			}
		}
	}(reader, writer)

	go func(r *io.PipeReader) {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			log.Infof("Sending log: [%v]", text)
			message := common.Message{
				Key:  key,
				Type: common.Body,
				Body: text,
			}
			response <- message
		}
		if err := scanner.Err(); err != nil {
			log.Errorf("Error with the container log scanner.", err)
		}
		log.Info("Leaving the scanner.")
	}(reader)

	err = client.Logs(logopts)
	if err != nil {
		log.Errorf("Problem getting logs for container [%v]", err)
		return
	}
	log.Info("Leaving the method.")
}
