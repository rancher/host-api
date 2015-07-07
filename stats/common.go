package stats

import (
	"fmt"
	"os"
	"strings"

	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/rancherio/host-api/config"
)

func pathParts(path string) []string {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	return strings.Split(path, "/")
}

func resolveContainer(id string) (string, error) {
	if id == "" {
		return "", nil
	}

	client, err := dockerClient.NewClient(config.Config.DockerUrl)
	if err != nil {
		return "", err
	}

	container, err := client.InspectContainer(id)
	if err != nil || container == nil {
		return "", err
	}

	if useSystemd() {
		return fmt.Sprintf("system.slice/docker-%s.scope", container.ID), nil
	} else {
		return fmt.Sprintf("docker/%s", container.ID), nil
	}
}

func useSystemd() bool {
	s, err := os.Stat("/run/systemd/system")
	if err != nil || !s.IsDir() {
		return false
	}

	return true
}
