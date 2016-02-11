package events

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/locks"
	rclient "github.com/rancherio/go-rancher/client"
)

type SendToRancherHandler struct {
	client   SimpleDockerClient
	rancher  *rclient.RancherClient
	hostUuid string
}

func (h *SendToRancherHandler) Handle(event *docker.APIEvents) error {
	// rancher_state_watcher sends a simulated event to the event router to initiate ip injection.
	// This event should not be sent.
	if event.From == simulatedEvent {
		return nil
	}

	// Note: event.ID == container's ID
	lock := locks.Lock(event.Status + event.ID)
	if lock == nil {
		log.Debugf("Container locked. Can't run SendToRancherHandler. Event: [%s], ID: [%s]", event.Status, event.ID)
		return nil
	}
	defer lock.Unlock()

	container, err := h.client.InspectContainer(event.ID)
	if err != nil {
		if _, ok := err.(*docker.NoSuchContainer); !ok {
			return err
		}
	}

	containerEvent := &rclient.ContainerEvent{
		ExternalStatus:    event.Status,
		ExternalId:        event.ID,
		ExternalFrom:      event.From,
		ExternalTimestamp: int64(event.Time),
		ReportedHostUuid:  h.hostUuid,
	}
	if container != nil {
		containerEvent.DockerInspect = container

	}

	if _, err := h.rancher.ContainerEvent.Create(containerEvent); err != nil {
		return err
	}

	return nil
}
