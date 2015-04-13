package events

import (
	"github.com/fsouza/go-dockerclient"
)

type SimpleDockerClient interface {
	InspectContainer(id string) (*docker.Container, error)
}
