package events

import (
	"encoding/json"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/locks"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

const RancherIPKey = "RANCHER_IP="

type StartHandler struct {
	Client SimpleDockerClient
}

func (h *StartHandler) Handle(event *docker.APIEvents) error {
	// Note: event.ID == container's ID
	lock := locks.Lock("start." + event.ID)
	if lock == nil {
		log.Infof("Container locked. Can't run StartHandler. ID: [%s]", event.ID)
		return nil
	}
	defer lock.Unlock()

	c, err := h.Client.InspectContainer(event.ID)
	if err != nil {
		return err
	}

	rancherIP := h.getRancherIP(c)
	if rancherIP == "" {
		return nil
	}

	if !c.State.Running {
		log.Infof("Container [%s] not running. Can't assign IP [%s].", c.ID, rancherIP)
		return nil
	}

	pid := c.State.Pid
	log.Infof("Assigning IP [%s], ContainerId [%s], Pid [%v]", rancherIP, event.ID, pid)
	if err := configureIP(strconv.Itoa(pid), rancherIP); err != nil {
		// If it stopped running, don't return error
		c, inspectErr := h.Client.InspectContainer(event.ID)
		if inspectErr != nil {
			log.Warn("Failed to inspect container: ", event.ID, inspectErr)
			return err
		}

		if !c.State.Running {
			log.Infof("Container [%s] not running. Cant assign IP [%s].", c.ID, rancherIP)
			return nil
		}

		return err
	}

	return nil
}

func (h *StartHandler) getRancherIP(c *docker.Container) string {
	for _, env := range c.Config.Env {
		if strings.HasPrefix(env, RancherIPKey) {
			return strings.TrimPrefix(env, RancherIPKey)
		}
	}

	filePath := path.Join(getContainerStateDir(), c.ID)
	if _, err := os.Stat(filePath); err == nil {
		file, e := ioutil.ReadFile(filePath)
		if e != nil {
			log.Errorf("Error reading file for container %s: %v", c.ID, e)
		}
		var instanceData instance
		json.Unmarshal(file, &instanceData)

		if len(instanceData.Nics) > 0 {
			nic := instanceData.Nics[0]
			var ipData ipAddress
			for _, i := range nic.IpAddresses {
				if i.Role == "primary" {
					ipData = i
					break
				}
			}
			if ipData.Address != "" {
				return ipData.Address + "/" + strconv.Itoa(ipData.Subnet.CidrSize)
			}
		}
	}

	return ""
}

func configureIP(pid string, ip string) error {
	command := buildCommand(pid, ip)

	output, err := command.CombinedOutput()
	log.Debugf(string(output))
	if err != nil {
		return err
	}

	return nil
}

var buildCommand = func(pid string, ip string) *exec.Cmd {
	return exec.Command("net-util.sh", "-p", pid, "-i", ip)
}
