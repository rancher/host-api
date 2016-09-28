package cadvisor

import "github.com/rancher/host-api/cadvisor/docker"

func (m *ConcreteManager) GetDockerFactory() (error) {
	factory, err := docker.Register(m, m.fsInfo, m.ignoreMetrics)
	if err != nil {
		return err
	}
	docker.DockerFactory = factory
	return nil
}
