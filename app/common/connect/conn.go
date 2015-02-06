package connect

import (
	"github.com/gorilla/websocket"
	"net/http"
	"strings"
)

type connection struct {
	webConn *websocket.Conn
	rw      http.ResponseWriter
}

func (conn *connection) Write(data []byte) (int, error) {
	if conn.webConn == nil {
		count, err := conn.rw.Write(data)
		if err != nil {
			return count, err
		}

		if flusher, ok := conn.rw.(http.Flusher); ok {
			flusher.Flush()
		}
	} else {
		err := conn.webConn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			return 0, err
		}
	}

	return len(data), nil
}

func (conn *connection) IsContinuous() bool {
	return conn.webConn != nil
}

func GetConnection(rw http.ResponseWriter, req *http.Request) (*connection, error) {
	var upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}

	upgrader.CheckOrigin = func(r *http.Request) bool {
		// allow all connections by default
		return true
	}

	conn := &connection{}

	for _, v := range req.Header["Connection"] {
		for _, i := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(i), "upgrade") {
				webConn, err := upgrader.Upgrade(rw, req, nil)
				if err != nil {
					return nil, err
				}

				conn.webConn = webConn
				return conn, nil
			}
		}
	}

	conn.rw = rw
	return conn, nil
}
