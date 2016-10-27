package events

import (
	"github.com/docker/docker/client"
)

const (
	defaultApiVersion = "1.22"
)

func NewDockerClient() (*client.Client, error) {
	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	cli.UpdateClientVersion(defaultApiVersion)
	return cli, nil
}
