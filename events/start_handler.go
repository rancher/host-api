package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/locks"
)

const RancherIPLabelKey = "io.rancher.container.ip"
const RancherSystemLabelKey = "io.rancher.container.system"
const RancherIPEnvKey = "RANCHER_IP="
const RancherNameserver = "169.254.169.250"
const RancherDomain = "rancher.internal"

type StartHandler struct {
	Client            SimpleDockerClient
	ContainerStateDir string
}

func setupResolvConf(container *docker.Container) error {
	if _, ok := container.Config.Labels[RancherSystemLabelKey]; ok {
		return nil
	}

	p := container.ResolvConfPath
	input, err := os.Open(p)
	if err != nil {
		return err
	}

	defer input.Close()

	var buffer bytes.Buffer
	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.Contains(text, RancherNameserver) {
			continue
		}

		if strings.HasPrefix(text, "nameserver") {
			text = "# " + text
		}

		if strings.HasPrefix(text, "search") && !strings.Contains(text, RancherDomain) {
			text = text + " " + RancherDomain
		}

		if _, err := buffer.Write([]byte(text)); err != nil {
			return err
		}

		if _, err := buffer.Write([]byte("\n")); err != nil {
			return err
		}
	}

	buffer.Write([]byte("nameserver "))
	buffer.Write([]byte(RancherNameserver))
	buffer.Write([]byte("\n"))

	input.Close()
	return ioutil.WriteFile(p, buffer.Bytes(), 0666)
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

	rancherIP, err := h.getRancherIP(c)
	if err != nil {
		return err
	}
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
			log.Infof("Container [%s] not running. Can't assign IP [%s].", c.ID, rancherIP)
			return nil
		}

		return err
	}

	return setupResolvConf(c)
}

func (h *StartHandler) getRancherIP(c *docker.Container) (string, error) {
	if ip, ok := c.Config.Labels[RancherIPLabelKey]; ok {
		return ip, nil
	}
	for _, env := range c.Config.Env {
		if strings.HasPrefix(env, RancherIPEnvKey) {
			return strings.TrimPrefix(env, RancherIPEnvKey), nil
		}
	}

	filePath := path.Join(h.ContainerStateDir, c.ID)
	if _, err := os.Stat(filePath); err == nil {
		file, e := ioutil.ReadFile(filePath)
		if e != nil {
			return "", fmt.Errorf("Error reading file for container %s: %v", c.ID, e)
		}
		var instanceData instance
		jerr := json.Unmarshal(file, &instanceData)
		if jerr != nil {
			return "", fmt.Errorf("Error unmarshalling json for container %s: %v", c.ID, jerr)
		}

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
				return ipData.Address + "/" + strconv.Itoa(ipData.Subnet.CidrSize), nil
			}
		}
	}

	return "", nil
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

type subnet struct {
	CidrSize int
}

type ipAddress struct {
	Address string
	Subnet  subnet
	Role    string
}

type nic struct {
	IpAddresses []ipAddress `json:"ipAddresses"`
}

type instance struct {
	Nics []nic
}
