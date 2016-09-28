package stats

import (
	"fmt"
	"os"
	"strings"

	jwt "github.com/dgrijalva/jwt-go"
	dockerClient "github.com/fsouza/go-dockerclient"
	info "github.com/google/cadvisor/info/v1"
	"github.com/rancher/host-api/config"
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

func parseRequestToken(tokenString string, parsedPublicKey interface{}) (*jwt.Token, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("No JWT token provided")
	}

	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return parsedPublicKey, nil
	})
}

func getContainerStats(count int, id string) (*info.ContainerInfo, error) {
	var containerInfo *info.ContainerInfo
	if id == "" {
		i, err := GetRootContainerInfo(count)
		if err != nil {
			return nil, err
		}
		containerInfo = i

	} else {
		id = "/docker/" + id
		i, err := GetDockerContainerInfo(id, count)
		if err != nil {
			// Try old approach
			container, err := resolveContainer(id)
			if err != nil {
				return nil, err
			}
			i, err := GetDockerContainerInfo(container, count)
			if err != nil {
				return nil, err
			}
			containerInfo = i
		} else {
			containerInfo = i
		}
	}

	return containerInfo, nil
}
