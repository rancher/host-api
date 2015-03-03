package events

import (
	"github.com/fsouza/go-dockerclient"
	"os"
	"testing"
)

func useEnvVars() bool {
	return os.Getenv("CATTLE_DOCKER_USE_BOOT2DOCKER") == "true"
}

func createContainer(client *docker.Client) (*docker.Container, error) {
	opts := docker.CreateContainerOptions{Config: &docker.Config{Image: "tianon/true"}}
	return client.CreateContainer(opts)
}

func createNetTestContainer(client *docker.Client, ip string) (*docker.Container, error) {
	env := []string{"RANCHER_IP=" + ip}
	config := &docker.Config{
		Image:     "ubuntu",
		Env:       env,
		OpenStdin: true,
		StdinOnce: false,
	}
	opts := docker.CreateContainerOptions{Config: config}
	return client.CreateContainer(opts)
}

func pullTestImages(client *docker.Client) {
	imageOptions := docker.PullImageOptions{
		Repository: "tianon/true",
	}
	imageAuth := docker.AuthConfiguration{}
	client.PullImage(imageOptions, imageAuth)
	imageOptions.Repository = "ubuntu"
	client.PullImage(imageOptions, imageAuth)
}

func TestMain(m *testing.M) {
	client, _ := NewDockerClient(useEnvVars())
	pullTestImages(client)
	result := m.Run()
	os.Exit(result)
}
