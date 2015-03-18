package events

import (
	"github.com/fsouza/go-dockerclient"
	rclient "github.com/rancherio/go-rancher/client"
	"testing"
	"time"
)

func TestProcessDockerEvents(t *testing.T) {
	processor := NewDockerEventsProcessor(10)

	// Test-friendly docker client
	useEnvVars := useEnvVars()
	dockerClient, err := NewDockerClient(useEnvVars)
	if err != nil {
		t.Fatal(err)
	}
	processor.getDockerClient = func() (*docker.Client, error) {
		return dockerClient, nil
	}

	// Mock Handler
	handledEvents := make(chan *docker.APIEvents, 10)
	hFn := func(event *docker.APIEvents) error {
		handledEvents <- event
		return nil
	}
	handler := &testHandler{
		handlerFunc: hFn,
	}
	processor.getHandlers = func(dockerClient *docker.Client,
		rancherClient *rclient.RancherClient) (map[string]Handler, error) {
		return map[string]Handler{"start": handler}, nil
	}

	// Create pre-existing containers before starting event listener
	preexistRunning, _ := createNetTestContainer(dockerClient, "10.1.2.3")
	defer func() {
		if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: preexistRunning.ID, Force: true,
			RemoveVolumes: true}); err != nil {
			t.Fatal(err)
		}
	}()
	if err := dockerClient.StartContainer(preexistRunning.ID, &docker.HostConfig{}); err != nil {
		t.Fatal(err)
	}
	preexistPaused, _ := createNetTestContainer(dockerClient, "10.1.2.3")
	defer func() {
		dockerClient.UnpauseContainer(preexistPaused.ID)
		if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: preexistPaused.ID, Force: true,
			RemoveVolumes: true}); err != nil {
			t.Fatal(err)
		}
	}()
	if err := dockerClient.StartContainer(preexistPaused.ID, &docker.HostConfig{}); err != nil {
		t.Fatal(err)
	}
	dockerClient.PauseContainer(preexistPaused.ID)

	if err := processor.Process(); err != nil {
		t.Fatal(err)
	}

	waitingOnRunning := true
	waitingOnPaused := true
	for waitingOnRunning || waitingOnPaused {
		select {
		case e := <-handledEvents:
			if e.ID == preexistRunning.ID {
				waitingOnRunning = false
			}
			if e.ID == preexistPaused.ID {
				waitingOnPaused = false
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("Never received event for preexisting container [%v]", preexistRunning.ID)
		}
	}
}

func TestGetHandlers(t *testing.T) {
	dockerClient := prep(t)
	handlers, err := getHandlersFn(dockerClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Cattle API config params not set, so CreateHandler shouldn't get configured
	if len(handlers) != 1 {
		t.Fatalf("Expected 1 configured hanlder: %v", handlers)
	}

	// RancherClient is not nil, so CreateHandler should be configured
	handlers, err = getHandlersFn(dockerClient, &rclient.RancherClient{})
	if err != nil {
		t.Fatal(err)
	}
	if len(handlers) != 2 {
		t.Fatalf("Expected 2 configured hanlders: %v, %#v", len(handlers), handlers)
	}
}
