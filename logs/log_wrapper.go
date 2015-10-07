package logs

import (
	"io"
)

var messageSeparator = []byte("[RANLOGS]")
var messageSeparatorLength = len(messageSeparator)
var messageOffset = 3 + messageSeparatorLength

var stdoutPrefix = []byte("01 ")
var stderrPrefix = []byte("02 ")
var bothPrefix = []byte("00 ")

type stdoutWriter struct {
	writer io.Writer
}

type stderrorWriter struct {
	writer io.Writer
}
type stdbothWriter struct {
	writer io.Writer
}

func wrapMessage(writer io.Writer, prefix []byte, message []byte) (n int, err error) {
	msg := append(prefix, message...)
	msg = append(msg, messageSeparator...)
	n, err = writer.Write(msg)
	return n - messageOffset, err
}

func (w stdoutWriter) Write(message []byte) (n int, err error) {
	return wrapMessage(w.writer, stdoutPrefix, message)
}

func (w stderrorWriter) Write(message []byte) (n int, err error) {
	return wrapMessage(w.writer, stderrPrefix, message)
}

func (w stdbothWriter) Write(message []byte) (n int, err error) {
	return wrapMessage(w.writer, bothPrefix, message)
}
