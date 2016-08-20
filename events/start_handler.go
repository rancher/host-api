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
	"github.com/rancher/event-subscriber/locks"
)

const RancherIPLabelKey = "io.rancher.container.ip"
const RancherSystemLabelKey = "io.rancher.container.system"
const RancherIPEnvKey = "RANCHER_IP="
const RancherNameserver = "169.254.169.250"
const RancherDomain = "rancher.internal"
const RancherDns = "io.rancher.container.dns"
const RancherVM = "io.rancher.vm"

type StartHandler struct {
	Client            SimpleDockerClient
	ContainerStateDir string
}

func getDnsSearch(container *docker.Container) []string {
	var defaultDomains []string
	var svcNameSpace string
	var stackNameSpace string

	//from labels - for upgraded systems
	if container.Config.Labels != nil {
		if value, ok := container.Config.Labels["io.rancher.stack_service.name"]; ok {
			splitted := strings.Split(value, "/")
			svc := strings.ToLower(splitted[1])
			stack := strings.ToLower(splitted[0])
			svcNameSpace = svc + "." + stack + "." + RancherDomain
			stackNameSpace = stack + "." + RancherDomain
			defaultDomains = append(defaultDomains, svcNameSpace)
			defaultDomains = append(defaultDomains, stackNameSpace)
		}
	}

	//from search domains
	if container.HostConfig.DNSSearch != nil {
		for _, domain := range container.HostConfig.DNSSearch {
			if domain != svcNameSpace && domain != stackNameSpace {
				defaultDomains = append(defaultDomains, domain)
			}
		}
	}

	// default rancher domain
	defaultDomains = append(defaultDomains, RancherDomain)
	return defaultDomains
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
	searchSet := false
	nameserverSet := false
	for scanner.Scan() {
		text := scanner.Text()

		if strings.Contains(text, RancherNameserver) {
			nameserverSet = true
		} else if strings.HasPrefix(text, "nameserver") {
			text = "# " + text
		}

		if strings.HasPrefix(text, "search") {
			for _, domain := range getDnsSearch(container) {
				if strings.Contains(text, " "+domain) {
					continue
				}
				text = text + " " + domain
			}
			searchSet = true
		}

		if _, err := buffer.Write([]byte(text)); err != nil {
			return err
		}

		if _, err := buffer.Write([]byte("\n")); err != nil {
			return err
		}
	}

	if !searchSet {
		buffer.Write([]byte("search " + strings.ToLower(strings.Join(getDnsSearch(container), " "))))
		buffer.Write([]byte("\n"))
	}

	if !nameserverSet {
		buffer.Write([]byte("nameserver "))
		buffer.Write([]byte(RancherNameserver))
		buffer.Write([]byte("\n"))
	}

	input.Close()
	return ioutil.WriteFile(p, buffer.Bytes(), 0666)
}

func (h *StartHandler) Handle(event *docker.APIEvents) error {
	// Note: event.ID == container's ID
	lock := locks.Lock("start." + event.ID)
	if lock == nil {
		log.Debugf("Container locked. Can't run StartHandler. ID: [%s]", event.ID)
		return nil
	}
	defer lock.Unlock()

	c, err := h.Client.InspectContainer(event.ID)
	if err != nil {
		return err
	}

	if c.Config.Labels[RancherVM] == "true" {
		return nil
	}

	rancherIP, err := h.getRancherIP(c)
	if err != nil {
		return err
	}

	if rancherIP == "" {
		if c.Config.Labels[RancherDns] == "true" {
			return setupResolvConf(c)
		}
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

	if c.Config.Labels[RancherDns] == "false" {
		return nil
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
