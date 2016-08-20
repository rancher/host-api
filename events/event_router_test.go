package events

import (
	_ "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"testing"
	"time"
)

type testHandler struct {
	handledEvents chan *docker.APIEvents
	t             *testing.T
	handlerFunc   func(event *docker.APIEvents) error
}

func (th *testHandler) Handle(event *docker.APIEvents) error {
	return th.handlerFunc(event)
}

func TestEventRouter(t *testing.T) {
	handledEvents := make(chan *docker.APIEvents, 10)
	hFn := func(event *docker.APIEvents) error {
		handledEvents <- event
		return nil
	}

	handler := &testHandler{
		handlerFunc: hFn,
	}
	handlers := map[string][]Handler{"create": {handler}}
	dockerClient, _ := NewDockerClient()
	router, _ := NewEventRouter(5, 5, dockerClient, handlers)
	defer router.Stop()
	router.Start()

	createCount := 2
	spinupContainers(createCount, dockerClient, t)

	receivedCount := 0
	for receivedCount != createCount {
		select {
		case <-handledEvents:
			receivedCount++
		case <-time.After(10 * time.Second):
			t.Fatalf("Timed out waiting on docker events.")
		}
	}

	if receivedCount != 2 {
		t.Fatalf("Received [%v] events", receivedCount)
	}
}

func TestWorkerTimeout(t *testing.T) {
	// This test proves the worker timeout and retry logic is working properly by making
	// the handler take longer than the worker timeout and then asserting that all events
	// were still handled.
	handledEvents := make(chan *docker.APIEvents, 10)
	hFn := func(event *docker.APIEvents) error {
		time.Sleep(20 * time.Millisecond)
		handledEvents <- event
		return nil
	}
	handler := &testHandler{
		handlerFunc: hFn,
	}

	handlers := map[string][]Handler{"create": {handler}}

	dockerClient, _ := NewDockerClient()
	router, _ := NewEventRouter(1, 1, dockerClient, handlers)
	router.workerTimeout = 10 * time.Millisecond
	defer router.Stop()
	router.Start()

	createCount := 2
	spinupContainers(createCount, dockerClient, t)

	receivedCount := 0
	timeoutCount := 0
	for receivedCount != createCount {
		select {
		case <-handledEvents:
			receivedCount++
		case <-time.After(10 * time.Millisecond):
			timeoutCount++
			if timeoutCount > 100 {
				t.Fatalf("Timed out waiting on docker events.")
			}
		}
	}

	if receivedCount != 2 {
		t.Fatalf("Received [%v] events", receivedCount)
	}
}

func spinupContainers(createCount int, dockerClient *docker.Client, t *testing.T) {
	for i := 0; i < createCount; i++ {
		c, err := createContainer(dockerClient)
		if err != nil {
			t.Fatalf("Failure: %v", err)
		}

		removeOpts := docker.RemoveContainerOptions{ID: c.ID}
		if err := dockerClient.RemoveContainer(removeOpts); err != nil {
			t.Fatalf("Failure", err)
		}
	}
}
