package events

import (
	"bufio"
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancher/event-subscriber/locks"
	"io"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

var containerJsonTemplate = `{"nics": [{"ipAddresses": [{"address": "%s", 
		"role": "primary", "subnet": {"cidrSize": 16}}]}]}`

func TestStartHandlerHappyPath(t *testing.T) {
	dockerClient := prep(t)

	injectedIp := "10.1.2.3"
	c, err := createNetTestContainer(dockerClient, injectedIp)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})

	assertIpInject(injectedIp, c, dockerClient, t)
}

func TestStartHandlerEnvVar(t *testing.T) {
	dockerClient := prep(t)

	injectedIp := "10.1.2.3"
	c, err := createNetTestContainerNoLabel(dockerClient, injectedIp)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})

	assertIpInject(injectedIp, c, dockerClient, t)
}

func assertIpInject(injectedIp string, c *docker.Container, dockerClient *docker.Client, t *testing.T) {
	if err := dockerClient.StartContainer(c.ID, &docker.HostConfig{}); err != nil {
		t.Fatal(err)
	}

	handler := &StartHandler{Client: dockerClient}
	event := &docker.APIEvents{ID: c.ID}

	if err := handler.Handle(event); err != nil {
		t.Fatal(err)
	}

	ok, err := assertCheckCmdOutput(injectedIp, c, dockerClient, []string{"ip", "addr", "show", "eth0"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Ip wasn't injected.")
	}
}

func TestStartHandlerContainerNotRunning(t *testing.T) {
	dockerClient := prep(t)

	injectedIp := "10.1.2.3"
	c, _ := createNetTestContainer(dockerClient, injectedIp)
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})
	// Note we aren't starting the container in this test

	handler := &StartHandler{Client: dockerClient}
	event := &docker.APIEvents{ID: c.ID}

	if err := handler.Handle(event); err != nil {
		t.Fatal(err)
	}
}

type mockClient struct {
	inspect      func(id string) (*docker.Container, error)
	container    *docker.Container
	inspectCount int
}

func (m *mockClient) InspectContainer(id string) (*docker.Container, error) {
	return m.inspect(id)
}

func TestStoppedRunning(t *testing.T) {
	injectedIp := "10.1.2.3"
	id := "some id"
	conf := &docker.Config{Labels: map[string]string{"io.rancher.container.system": "fakeSysContainer"}, Env: []string{"RANCHER_IP=" + injectedIp}}
	state := docker.State{Pid: 123, Running: true}
	container := &docker.Container{ID: id, Config: conf, State: state}

	var inspectCount int
	mockedInspect := func(id string) (*docker.Container, error) {
		if inspectCount == 1 {
			container.State.Running = false
		}
		inspectCount += 1
		return container, nil
	}
	mock := &mockClient{inspect: mockedInspect}

	handler := &StartHandler{Client: mock}
	event := &docker.APIEvents{ID: id}
	if err := handler.Handle(event); err != nil {
		t.Fatal("Error should not have been returned.")
	}
}

func TestLocking(t *testing.T) {
	dockerClient := prep(t)

	eventId := "fake id"
	lock := locks.Lock("start." + eventId)

	handler := &StartHandler{Client: dockerClient}
	event := &docker.APIEvents{ID: eventId}

	if err := handler.Handle(event); err != nil {
		// If the lock didn't work, this would fail because of the fake id.
		t.Fatal(err)
	}

	lock.Unlock()

	if err := handler.Handle(event); err == nil {
		// Unlocked, should return error
		t.Fatal(err)
	}
}

func TestIpFromFile(t *testing.T) {
	purgeDir(t)
	mkTestDir(t)
	dockerClient := prep(t)

	injectedIp := "10.1.2.3"
	c, err := createNetTestContainer(dockerClient, "")
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})

	stateDir := makeContainerFile(t, c.ID, fmt.Sprintf(containerJsonTemplate, injectedIp))

	if err := dockerClient.StartContainer(c.ID, &docker.HostConfig{}); err != nil {
		t.Fatal(err)
	}

	handler := &StartHandler{Client: dockerClient,
		ContainerStateDir: stateDir}
	event := &docker.APIEvents{ID: c.ID}

	if err := handler.Handle(event); err != nil {
		t.Fatal(err)
	}

	ok, err := assertCheckCmdOutput(injectedIp, c, dockerClient, []string{"ip", "addr", "show", "eth0"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Ip wasn't injected.")
	}
}

func TestIpFromFileFailure(t *testing.T) {
	purgeDir(t)
	mkTestDir(t)
	dockerClient := prep(t)

	c, err := createNetTestContainer(dockerClient, "")
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})

	stateDir := makeContainerFile(t, c.ID, `{"nics": [{"ipAddresses": [{`)

	if err := dockerClient.StartContainer(c.ID, &docker.HostConfig{}); err != nil {
		t.Fatal(err)
	}

	handler := &StartHandler{Client: dockerClient,
		ContainerStateDir: stateDir}
	event := &docker.APIEvents{ID: c.ID}

	if err := handler.Handle(event); err == nil {
		t.Fatal("Expected to get json unmarshalling error.")
	}
}

func TestDnsWithLabel(t *testing.T) {
	dockerClient := prep(t)
	injectedIp := ""
	labels := make(map[string]string)
	labels["io.rancher.container.dns"] = "true"
	c, err := createTestContainer(dockerClient, injectedIp, labels, false)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})

	assertIpInject(injectedIp, c, dockerClient, t)

	ok, err := assertCheckCmdOutput("169.254.169.250", c, dockerClient, []string{"cat", "/etc/resolv.conf"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Dns wasn't set with label directive.")
	}
}

func TestDnsWithoutLabel(t *testing.T) {
	dockerClient := prep(t)
	injectedIp := ""
	c, err := createTestContainer(dockerClient, injectedIp, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})

	assertIpInject(injectedIp, c, dockerClient, t)

	ok, err := assertCheckCmdOutput("169.254.169.250", c, dockerClient, []string{"cat", "/etc/resolv.conf"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Dns was reset with no label directive.")
	}
}

func makeContainerFile(t *testing.T, id string, containerJson string) string {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	stateDir := path.Join(currentDir, watchTestDir)
	filePath := path.Join(stateDir, id)
	json := []byte(containerJson)
	if err := ioutil.WriteFile(filePath, json, 0644); err != nil {
		t.Fatal(err)
	}
	return stateDir
}

func assertCheckCmdOutput(inputToCheck string, c *docker.Container, dockerClient *docker.Client, cmd []string, ifExists bool) (bool, error) {
	createExecConf := docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Cmd:          cmd,
		Container:    c.ID,
	}
	createdExec, err := dockerClient.CreateExec(createExecConf)
	if err != nil {
		return false, err
	}

	reader, writer := io.Pipe()
	startExecConf := docker.StartExecOptions{
		OutputStream: writer,
		Detach:       false,
		Tty:          false,
		RawTerminal:  false,
	}

	go dockerClient.StartExec(createdExec.ID, startExecConf)

	lines := func(reader io.Reader) chan string {
		lines := make(chan string)
		go func(r io.Reader) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				lines <- scanner.Text()
			}
		}(reader)
		return lines
	}(reader)

	timer := time.NewTimer(1 * time.Second)
	found := false
	keepReading := true
	for keepReading {
		select {
		case line := <-lines:
			if strings.Contains(line, inputToCheck) == ifExists {
				found = true
				keepReading = false
				break
			}
		case <-timer.C:
			keepReading = false
			break
		}
	}

	return found, nil
}
