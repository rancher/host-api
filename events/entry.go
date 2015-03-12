package events

import (
	"github.com/fsouza/go-dockerclient"
)

func ProcessDockerEvents(poolSize int) error {
	dockerClient, err := getDockerClient()
	if err != nil {
		return err
	}

	handlers := getHandlers(dockerClient)
	router, err := NewEventRouter(poolSize, poolSize, dockerClient, handlers)
	if err != nil {
		return err
	}
	router.Start()

	listOpts := docker.ListContainersOptions{
		All:     true,
		Filters: map[string][]string{"status": {"paused", "running"}},
	}
	containers, err := dockerClient.ListContainers(listOpts)
	if err != nil {
		return err
	}

	for _, c := range containers {
		event := &docker.APIEvents{
			ID:     c.ID,
			Status: "start",
		}
		router.listener <- event
	}
	return nil
}

var getDockerClient = func() (*docker.Client, error) {
	return NewDockerClient(false)
}

var getHandlers = func(dockerClient *docker.Client) map[string]Handler {
	handler := &StartHandler{
		Client: dockerClient,
	}
	handlers := map[string]Handler{"start": handler}

	return handlers
}
