package proxy

import (
	"encoding/json"
	"io"
	"net/http"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/rancherio/websocket-proxy/backend"
	"github.com/rancherio/websocket-proxy/common"
)

type Handler struct {
}

func (s *Handler) Handle(key string, initialMessage string, incomingMessages <-chan string, response chan<- common.Message) {
	defer backend.SignalHandlerClosed(key, response)
	log := log.WithField("url", initialMessage)

	message, err := readMessage(incomingMessages)
	if err != nil {
		log.WithField("error", err).Error("Invalid content")
		return
	}

	req, err := http.NewRequest(message.Method, message.URL, &HttpReader{
		Buffered: message.Body,
		Chan:     incomingMessages,
		EOF:      message.EOF,
	})
	if err != nil {
		log.WithField("error", err).Error("Failed to create request")
		return
	}
	req.Host = message.Host
	req.Header = http.Header(message.Headers)

	client := http.Client{}
	client.Timeout = 60 * time.Second

	resp, err := client.Do(req)
	if err != nil {
		log.WithField("error", err).Error("Failed to make request")
		return
	}
	defer resp.Body.Close()

	httpResponseMessage := common.HttpMessage{
		Code:    resp.StatusCode,
		Headers: map[string][]string(resp.Header),
	}

	httpWriter := &HttpWriter{
		Message:    httpResponseMessage,
		MessageKey: key,
		Chan:       response,
	}
	defer httpWriter.Close()

	if _, err := io.Copy(httpWriter, resp.Body); err != nil {
		log.WithField("error", err).Error("Failed to write body")
		return
	}
}

func readMessage(incomingMessages <-chan string) (*common.HttpMessage, error) {
	str := <-incomingMessages
	var message common.HttpMessage
	if err := json.Unmarshal([]byte(str), &message); err != nil {
		return nil, err
	}
	return &message, nil
}
