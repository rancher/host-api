package events

import (
	"github.com/fsouza/go-dockerclient"
	rclient "github.com/rancherio/go-rancher/client"
	"github.com/rancherio/host-api/config"
)

func ProcessDockerEvents(poolSize int) error {
	dockerClient, err := getDockerClient()
	if err != nil {
		return err
	}

	handlers, err := getHandlers(dockerClient)
	if err != nil {
		return err
	}

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

var getHandlers = func(dockerClient *docker.Client) (map[string]Handler, error) {

	handlers := map[string]Handler{}

	// Start Handler
	startHandler := &StartHandler{
		Client: dockerClient,
	}
	handlers["start"] = startHandler

	// Create Handler
	rancherClient, err := rancherClient()
	if err != nil {
		return nil, err
	}

	if rancherClient != nil {
		createHandler := &CreateHandler{
			client:   dockerClient,
			rancher:  rancherClient,
			hostUuid: getHostUuid(),
		}

		handlers["create"] = createHandler
	}

	return handlers, nil
}

var rancherClient = func() (*rclient.RancherClient, error) {
	apiUrl := config.Config.CattleUrl
	accessKey := config.Config.CattleAccessKey
	secretKey := config.Config.CattleSecretKey

	if apiUrl == "" || accessKey == "" || secretKey == "" {
		return nil, nil
	}

	apiClient, err := rclient.NewRancherClient(&rclient.ClientOpts{
		Url:       apiUrl,
		AccessKey: accessKey,
		SecretKey: secretKey,
	})
	if err != nil {
		return nil, err
	}
	return apiClient, nil
}

func getHostUuid() string {
	return config.Config.HostUuid
}
