package events

import (
	"github.com/fsouza/go-dockerclient"
	"os"
	"testing"
)

func TestEventListener(t *testing.T) {

}

func newDockerClient() (*docker.Client, error) {
	useDockerConnectEnvVars := os.Getenv("CATTLE_DOCKER_USE_BOOT2DOCKER") == "true"
	return DockerClient(useDockerConnectEnvVars)
}

func TestDockerClient(t *testing.T) {
	client, err := newDockerClient()
	if err != nil {
		t.Fatalf("Couldn't connect to docker.", err)
	}
	_, err = client.ListImages(docker.ListImagesOptions{All: false})
	if err != nil {
		t.Fatalf("Client not working.", err)
	}
}
