//+build windows

package events

import (
	"github.com/fsouza/go-dockerclient"
	"os"
	"fmt"
)

const (
	defaultApiVersion = "1.22"
)

func NewDockerClient() (*docker.Client, error) {
	apiVersion := getenv("DOCKER_API_VERSION", defaultApiVersion)
	endpoint := fmt.Sprintf("tcp://%v:2375", os.Getenv("DEFAULT_GATEWAY"))
	return docker.NewVersionedClient(endpoint, apiVersion)
}

func getenv(key string, defaultVal string) string {
	val := os.Getenv(key)
	if val == "" {
		val = defaultVal
	}
	return val
}