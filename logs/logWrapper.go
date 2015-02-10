package logs

import (
	"io"
)

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
	n, err = w.writer.Write(append([]byte("01 "), message...))
	return n - 3, err
}

func (w stderrorWriter) Write(message []byte) (n int, err error) {
	n, err = w.writer.Write(append([]byte("02 "), message...))
	return n - 3, err
}

func (w stdbothWriter) Write(message []byte) (n int, err error) {
	n, err = w.writer.Write(append([]byte("00 "), message...))
	return n - 3, err
}
