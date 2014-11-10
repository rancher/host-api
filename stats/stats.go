package stats

import (
	"encoding/json"
	"fmt"
	"github.com/rancherio/host-api/config"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/google/cadvisor/client"
	"github.com/google/cadvisor/info"
	"github.com/gorilla/mux"
	"io"
	"net/http"
	"time"
)

func GetStats(rw http.ResponseWriter, req *http.Request) error {
	id := mux.Vars(req)["id"]

	container, err := resolveContainer(id)
	if err != nil {
		return err
	}

	c, err := client.NewClient(config.Config.CAdvisorUrl)
	if err != nil {
		return err
	}

	conn, err := getConnection(rw, req)
	if err != nil {
		return err
	}

	count := 1
	if conn.IsContinuous() {
		count = config.Config.NumStats
	}

	for {
		machineInfo, err := c.MachineInfo()
		if err != nil {
			return err
		}

		memLimit := machineInfo.MemoryCapacity

		info, err := c.ContainerInfo(container, &info.ContainerInfoRequest{
			NumStats: count,
		})
		if err != nil {
			return err
		}

		err = writeStats(info, memLimit, conn)
		if err != nil {
			return err
		}

		if conn.IsContinuous() {
			time.Sleep(1 * time.Second)
			count = 1
		} else {
			break
		}
	}

	return nil
}

func writeStats(info *info.ContainerInfo, memLimit int64, writer io.Writer) error {
	for _, stat := range info.Stats {
		stat.Memory.Limit = uint64(memLimit)
		data, err := json.Marshal(stat)
		if err != nil {
			return err
		}

		writer.Write(data)
	}

	return nil
}

func resolveContainer(id string) (string, error) {
	if id == "" {
		return "", nil
	}

	client, err := dockerClient.NewClient(config.Config.DockerUrl)
	if err != nil {
		return "", err
	}

	container, err := client.InspectContainer(id)
	if err != nil || container == nil {
		return "", err
	}

	//if config.Config.Systemd {
	return fmt.Sprintf("system.slice/docker-%s.scope", container.ID), nil
	//} else {
	//return fmt.Sprintf("docker/%s", container.ID), nil
	//}
}
