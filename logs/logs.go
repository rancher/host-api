package logs

import (
	"bufio"
	"bytes"
	"io"
	"net/url"
	"strconv"
	_ "time"

	log "github.com/Sirupsen/logrus"
	dockerClient "github.com/fsouza/go-dockerclient"

	"github.com/rancher/websocket-proxy/backend"
	"github.com/rancher/websocket-proxy/common"

	// "github.com/rancherio/host-api/app/common/connect"
	"github.com/rancherio/host-api/auth"
	"github.com/rancherio/host-api/events"
)

type LogsHandler struct {
}

func (l *LogsHandler) Handle(key string, initialMessage string, incomingMessages <-chan string, response chan<- common.Message) {
	defer backend.SignalHandlerClosed(key, response)

	requestUrl, err := url.Parse(initialMessage)
	if err != nil {
		log.WithFields(log.Fields{"error": err, "url": initialMessage}).Error("Couldn't parse url.")
		return
	}
	tokenString := requestUrl.Query().Get("token")
	token, valid := auth.GetAndCheckToken(tokenString)
	if !valid {
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
		log.WithFields(log.Fields{"error": err}).Error("Couldn't get docker client.")
		return
	}

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
		scanner.Split(customSplit)
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

	// Returns an error, but ignoring it because it will always return an error when a streaming call is made.
	client.Logs(logopts)
}

func customSplit(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.Index(data, messageSeparator); i >= 0 {
		return i + messageSeparatorLength, data[0:i], nil
	}

	if atEOF {
		return len(data), data, nil
	}

	return 0, nil, nil
}
