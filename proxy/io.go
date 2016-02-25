package proxy

import (
	"encoding/json"
	"io"

	"github.com/rancherio/websocket-proxy/common"
)

type HttpWriter struct {
	headerWritten bool
	Message       common.HttpMessage
	MessageKey    string
	Chan          chan<- common.Message
}

func (h *HttpWriter) Write(bytes []byte) (n int, err error) {
	h.Message.Body = bytes
	if err := h.write(); err != nil {
		return 0, err
	}
	return len(bytes), nil
}

func (h *HttpWriter) write() error {
	bytes, err := json.Marshal(&h.Message)
	if err != nil {
		return err
	}
	h.Chan <- common.Message{
		Key:  h.MessageKey,
		Type: common.Body,
		Body: string(bytes),
	}
	h.Message = common.HttpMessage{}
	return nil
}

func (h *HttpWriter) Close() error {
	h.Message.EOF = true
	return h.write()
}

type HttpReader struct {
	Buffered []byte
	Chan     <-chan string
	EOF      bool
}

func (h *HttpReader) Read(bytes []byte) (int, error) {
	if len(h.Buffered) == 0 && !h.EOF {
		if err := h.read(); err != nil {
			return 0, err
		}
	}

	count := copy(bytes, h.Buffered)
	h.Buffered = h.Buffered[count:]

	if h.EOF {
		return count, io.EOF
	} else {
		return count, nil
	}
}

func (h *HttpReader) read() error {
	str, ok := <-h.Chan
	if !ok {
		return io.EOF
	}

	var message common.HttpMessage
	if err := json.Unmarshal([]byte(str), &message); err != nil {
		return err
	}

	h.Buffered = message.Body
	h.EOF = message.EOF
	return nil
}
