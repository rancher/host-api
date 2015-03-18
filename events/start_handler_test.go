package events

import (
	"bufio"
	"github.com/fsouza/go-dockerclient"
	"github.com/rancherio/go-machine-service/locks"
	"io"
	"strings"
	"testing"
	"time"
)

func TestStartHandlerHappyPath(t *testing.T) {
	dockerClient := prep(t)

	injectedIp := "10.1.2.3"
	c, err := createNetTestContainer(dockerClient, injectedIp)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true, RemoveVolumes: true})

	if err := dockerClient.StartContainer(c.ID, &docker.HostConfig{}); err != nil {
		t.Fatal(err)
	}

	handler := &StartHandler{Client: dockerClient}
	event := &docker.APIEvents{ID: c.ID}

	if err := handler.Handle(event); err != nil {
		t.Fatal(err)
	}

	ok, err := assertHasIp(injectedIp, c, dockerClient)
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
	conf := &docker.Config{Env: []string{"RANCHER_IP=" + injectedIp}}
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

func assertHasIp(injectedIp string, c *docker.Container, dockerClient *docker.Client) (bool, error) {
	createExecConf := docker.CreateExecOptions{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Cmd:          []string{"ip", "addr", "show", "eth0"},
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
	foundIp := false
	keepReading := true
	for keepReading {
		select {
		case line := <-lines:
			if strings.Contains(line, injectedIp) {
				foundIp = true
				keepReading = false
				break
			}
		case <-timer.C:
			keepReading = false
			break
		}
	}

	return foundIp, nil
}
