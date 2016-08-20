package events

import (
	"github.com/fsouza/go-dockerclient"
	rclient "github.com/rancher/go-rancher/client"
	"github.com/rancherio/host-api/config"
	"github.com/rancherio/host-api/util"
)

func NewDockerEventsProcessor(poolSize int) *DockerEventsProcessor {
	return &DockerEventsProcessor{
		poolSize:         poolSize,
		getDockerClient:  getDockerClientFn,
		getHandlers:      getHandlersFn,
		getRancherClient: util.GetRancherClient,
	}
}

type DockerEventsProcessor struct {
	poolSize         int
	getDockerClient  func() (*docker.Client, error)
	getHandlers      func(*docker.Client, *rclient.RancherClient) (map[string][]Handler, error)
	getRancherClient func() (*rclient.RancherClient, error)
}

func (de *DockerEventsProcessor) Process() error {
	dockerClient, err := de.getDockerClient()
	if err != nil {
		return err
	}

	rancherClient, err := de.getRancherClient()
	if err != nil {
		return err
	}

	handlers, err := de.getHandlers(dockerClient, rancherClient)
	if err != nil {
		return err
	}

	router, err := NewEventRouter(de.poolSize, de.poolSize, dockerClient, handlers)
	if err != nil {
		return err
	}
	router.Start()

	rancherStateWatcher := newRancherStateWatcher(router.listener, getContainerStateDir(), nil)
	go rancherStateWatcher.watch()

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
			From:   simulatedEvent,
		}
		router.listener <- event
	}
	return nil
}

func getDockerClientFn() (*docker.Client, error) {
	return NewDockerClient()
}

func getHandlersFn(dockerClient *docker.Client, rancherClient *rclient.RancherClient) (map[string][]Handler, error) {

	handlers := map[string][]Handler{}

	// Start Handler
	startHandler := &StartHandler{
		Client:            dockerClient,
		ContainerStateDir: getContainerStateDir(),
	}
	handlers["start"] = []Handler{startHandler}

	// Rancher Event Handler
	if rancherClient != nil {
		sendToRancherHandler := &SendToRancherHandler{
			client:   dockerClient,
			rancher:  rancherClient,
			hostUuid: getHostUuid(),
		}
		handlers["start"] = append(handlers["start"], sendToRancherHandler)
		handlers["stop"] = []Handler{sendToRancherHandler}
		handlers["die"] = []Handler{sendToRancherHandler}
		handlers["kill"] = []Handler{sendToRancherHandler}
		handlers["destroy"] = []Handler{sendToRancherHandler}
	}

	return handlers, nil
}

func getHostUuid() string {
	return config.Config.HostUuid
}

func getContainerStateDir() string {
	return config.Config.CattleStateDir
}
