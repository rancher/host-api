package events

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/locks"
	rclient "github.com/rancherio/go-rancher/client"
)

type CreateHandler struct {
	client   SimpleDockerClient
	rancher  *rclient.RancherClient
	hostUuid string
}

func (h *CreateHandler) Handle(event *docker.APIEvents) error {
	// Note: event.ID == container's ID
	lock := locks.Lock("create." + event.ID)
	if lock == nil {
		log.Warnf("Container locked. Can't run CreateHandler. ID: [%s]", event.ID)
		return nil
	}
	defer lock.Unlock()

	container, err := h.client.InspectContainer(event.ID)
	if err != nil {
		return err
	}

	containerEvent := &rclient.ContainerEvent{
		ExternalStatus:    event.Status,
		ExternalId:        event.ID,
		ExternalFrom:      event.From,
		ExternalTimestamp: int(event.Time),
		DockerInspect:     container,
		ReportedHostUuid:  h.hostUuid,
	}

	if _, err := h.rancher.ContainerEvent.Create(containerEvent); err != nil {
		return err
	}

	return nil
}
