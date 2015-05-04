package stats

import (
	"encoding/json"

	"fmt"
	"io"
	"net/http"
	"os"
	"time"
	sigar "github.com/cloudfoundry/gosigar"
	dockerClient "github.com/fsouza/go-dockerclient"
	"github.com/google/cadvisor/client"
	"github.com/google/cadvisor/info"
	"github.com/gorilla/mux"
	"github.com/rancherio/host-api/app/common/connect"
	"github.com/rancherio/host-api/config"
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

	conn, err := connect.GetConnection(rw, req)
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

		concreteSigar := &sigar.ConcreteSigar{}
		memLimit, err := concreteSigar.GetMem()

		if err != nil {
			return err
		}

		info, err := c.ContainerInfo(container, &info.ContainerInfoRequest{
			NumStats: count,
		})
		if err != nil {
			return err
		}

		err = writeStats(info, memLimit.Total, conn)
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

func writeStats(info *info.ContainerInfo, memLimit uint64, writer io.Writer) error {
	for _, stat := range info.Stats {
		stat.Memory.Limit = memLimit
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

	if useSystemd() {
		return fmt.Sprintf("system.slice/docker-%s.scope", container.ID), nil
	} else {
		return fmt.Sprintf("docker/%s", container.ID), nil
	}
}

func useSystemd() bool {
	s, err := os.Stat("/run/systemd/system")
	if err != nil || !s.IsDir() {
		return false
	}

	return true
}
