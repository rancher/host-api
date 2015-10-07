package logs

import (
	"io"
)

var messageSeparator = []byte("[RANLOGS]")
var messageOffset = 3 + len(messageSeparator)

type stdoutWriter struct {
	writer io.Writer
}

type stderrorWriter struct {
	writer io.Writer
}
type stdbothWriter struct {
	writer io.Writer
}

func (w stdoutWriter) Write(message []byte) (n int, err error) {
	msg := append([]byte("01 "), message...)
	msg = append(msg, messageSeparator...)
	n, err = w.writer.Write(msg)
	return n - messageOffset, err
}

func (w stderrorWriter) Write(message []byte) (n int, err error) {
	msg := append([]byte("02 "), message...)
	msg = append(msg, messageSeparator...)
	n, err = w.writer.Write(msg)
	return n - messageOffset, err
}

func (w stdbothWriter) Write(message []byte) (n int, err error) {
	msg := append([]byte("00 "), message...)
	msg = append(msg, messageSeparator...)
	n, err = w.writer.Write(msg)
	return n - messageOffset, err
}
