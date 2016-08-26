package events

import (
	"github.com/fsouza/go-dockerclient"
	rclient "github.com/rancher/go-rancher/client"
	"testing"
	"time"
)

func TestProcessDockerEvents(t *testing.T) {
	processor := NewDockerEventsProcessor(10)

	dockerClient, err := NewDockerClient()
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
		rancherClient *rclient.RancherClient) (map[string][]Handler, error) {
		return map[string][]Handler{"start": {handler}}, nil
	}

	// Create pre-existing containers before starting event listener
	preexistRunning, err := createNetTestContainer(dockerClient, "10.1.2.3")
	if err != nil {
		t.Fatal(err)
	}
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
			if e.From != simulatedEvent {
				t.Fatalf("Startup event was not marked as simulated. From value: [%v]", e.From)
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
	// Cattle API config params not set, so SendToRancherHandler shouldn't get configured
	if len(handlers) != 1 && len(handlers["start"]) != 1 {
		t.Fatalf("Expected 1 configured hanlder: %v", handlers)
	}

	// RancherClient is not nil, so SendToRancherHandler should be configured
	handlers, err = getHandlersFn(dockerClient, &rclient.RancherClient{})
	if err != nil {
		t.Fatal(err)
	}
	if len(handlers) != 5 {
		t.Fatalf("Expected 5 configured hanlders: %v, %#v", len(handlers), handlers)
	}
}
