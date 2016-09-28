package stats

import (
	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"github.com/golang/glog"
	"github.com/google/cadvisor/container"
	info "github.com/google/cadvisor/info/v1"
	"github.com/google/cadvisor/manager/watcher"
	"golang.org/x/net/context"
	"os"
)

func GetRootContainerInfo(count int) (*info.ContainerInfo, error) {
	inHostNamespace := false
	if _, err := os.Stat("/rootfs/proc"); os.IsNotExist(err) {
		inHostNamespace = true
	}
	handler, accept, err := container.NewContainerHandler("/", watcher.Raw, inHostNamespace)
	if err != nil {
		return nil, err
	}
	if !accept {
		// ignoring this container.
		glog.V(4).Infof("ignoring container %q", "/")
		return nil, err
	}
	var containerInfo info.ContainerInfo
	spec, err := handler.GetSpec()
	if err != nil {
		return nil, err
	}
	stats := []*info.ContainerStats{}
	containerInfo.Spec = spec
	for i := 0; i < count; i++ {
		stat, err := handler.GetStats()
		if err != nil {
			continue
		}
		stats = append(stats, stat)
	}
	containerInfo.Stats = stats
	return &containerInfo, nil
}

func GetDockerContainerInfo(id string, count int) (*info.ContainerInfo, error) {
	inHostNamespace := false
	if _, err := os.Stat("/rootfs/proc"); os.IsNotExist(err) {
		inHostNamespace = true
	}
	handler, accept, err := container.NewContainerHandler(id, watcher.Raw, inHostNamespace)
	if err != nil {
		return nil, err
	}
	if !accept {
		// ignoring this container.
		glog.V(4).Infof("ignoring container %q", "/")
		return nil, err
	}
	var containerInfo info.ContainerInfo
	spec, err := handler.GetSpec()
	if err != nil {
		return nil, err
	}
	stats := []*info.ContainerStats{}
	containerInfo.Spec = spec
	for i := 0; i < count; i++ {
		stat, err := handler.GetStats()
		if err != nil {
			continue
		}
		stats = append(stats, stat)
	}
	containerInfo.Stats = stats
	return &containerInfo, nil
}

func GetAllDockerContainers(count int) (map[string]*info.ContainerInfo, error) {
	inHostNamespace := false
	if _, err := os.Stat("/rootfs/proc"); os.IsNotExist(err) {
		inHostNamespace = true
	}

	cli, err := client.NewEnvClient()
	if err != nil {
		return nil, err
	}
	contList, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return nil, err
	}
	ret := map[string]*info.ContainerInfo{}
	for _, cont := range contList {
		handler, accept, err := container.NewContainerHandler(cont.ID, watcher.Raw, inHostNamespace)
		if err != nil {
			return nil, err
		}
		if !accept {
			// ignoring this container.
			glog.V(4).Infof("ignoring container %q", "/")
			return nil, err
		}
		var containerInfo info.ContainerInfo
		spec, err := handler.GetSpec()
		if err != nil {
			return nil, err
		}
		stats := []*info.ContainerStats{}
		containerInfo.Spec = spec
		for i := 0; i < count; i++ {
			stat, err := handler.GetStats()
			if err != nil {
				continue
			}
			stats = append(stats, stat)
		}
		containerInfo.Stats = stats
		ret[cont.ID] = &containerInfo
	}
	return ret, nil
}
