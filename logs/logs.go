package logs

import (
	"errors"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/rancherio/host-api/app/common/connect"
	"github.com/rancherio/host-api/auth"
	"github.com/rancherio/host-api/config"
	"net/http"
	"strconv"
)

func GetLogs(rw http.ResponseWriter, req *http.Request) error {

	token := auth.GetToken(req)

	if token == nil {
		return errors.New("No token stored in context.")
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

	client, err := dockerClient.NewClient(config.Config.DockerUrl)

	if err != nil {
		return err
	}

	conn, err := connect.GetConnection(rw, req)

	if err != nil {
		return err
	}
	logOut := stdoutWriter{conn}
	logError := stderrorWriter{conn}
	logBoth := stdbothWriter{conn}
	containerRef, err := client.InspectContainer(container)
	if err != nil {
		return err
	}

	if containerRef.Config.Tty == true {
		logopts := dockerClient.LogsOptions{
			Container:    container,
			OutputStream: logBoth,
			Follow:       follow,
			Stdout:       true,
			Stderr:       true,
			Timestamps:   true,
			Tail:         tail,
			RawTerminal:  true,
		}
		return client.Logs(logopts)
	} else {
		logopts := dockerClient.LogsOptions{
			Container:    container,
			OutputStream: logOut,
			ErrorStream:  logError,
			Follow:       follow,
			Stdout:       true,
			Stderr:       true,
			Timestamps:   true,
			Tail:         tail,
			RawTerminal:  false,
		}
		return client.Logs(logopts)
	}
}
