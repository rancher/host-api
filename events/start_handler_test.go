package events

import (
	"bufio"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/rancher/event-subscriber/locks"
	"golang.org/x/net/context"
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
	defer dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	assertIpInject(injectedIp, c, dockerClient, t)
}

func TestStartHandlerEnvVar(t *testing.T) {
	dockerClient := prep(t)

	injectedIp := "10.1.2.3"
	c, err := createNetTestContainerNoLabel(dockerClient, injectedIp)
	if err != nil {
		t.Fatal(err)
	}
	defer dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	assertIpInject(injectedIp, c, dockerClient, t)
}

func assertIpInject(injectedIp string, c types.ContainerJSON, dockerClient *client.Client, t *testing.T) {
	if err := dockerClient.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}

	handler := &StartHandler{Client: dockerClient}
	event := &events.Message{ID: c.ID}

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
	defer dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	// Note we aren't starting the container in this test

	handler := &StartHandler{Client: dockerClient}
	event := &events.Message{ID: c.ID}

	if err := handler.Handle(event); err != nil {
		t.Fatal(err)
	}
}

type mockClient struct {
	inspect      func(context context.Context, id string) (types.ContainerJSON, error)
	container    types.ContainerJSON
	inspectCount int
}

func (m *mockClient) ContainerInspect(context context.Context, id string) (types.ContainerJSON, error) {
	return m.inspect(context, id)
}

func TestStoppedRunning(t *testing.T) {
	injectedIp := "10.1.2.3"
	id := "some id"
	conf := &container.Config{Labels: map[string]string{"io.rancher.container.system": "fakeSysContainer"}, Env: []string{"RANCHER_IP=" + injectedIp}}
	state := &types.ContainerState{Pid: 123, Running: true}
	cont := types.ContainerJSON{
		Config: conf,
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:    id,
			State: state,
		},
	}

	var inspectCount int
	mockedInspect := func(context context.Context, id string) (types.ContainerJSON, error) {
		if inspectCount == 1 {
			cont.State.Running = false
		}
		inspectCount += 1
		return cont, nil
	}
	mock := &mockClient{inspect: mockedInspect}

	handler := &StartHandler{Client: mock}
	event := &events.Message{ID: id}
	if err := handler.Handle(event); err != nil {
		t.Fatal("Error should not have been returned.")
	}
}

func TestLocking(t *testing.T) {
	dockerClient := prep(t)

	eventId := "fake id"
	lock := locks.Lock("start." + eventId)

	handler := &StartHandler{Client: dockerClient}
	event := &events.Message{ID: eventId}

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
	defer dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	stateDir := makeContainerFile(t, c.ID, fmt.Sprintf(containerJsonTemplate, injectedIp))

	if err := dockerClient.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}
	handler := &StartHandler{Client: dockerClient,
		ContainerStateDir: stateDir}
	event := &events.Message{ID: c.ID}

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
	defer dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	stateDir := makeContainerFile(t, c.ID, `{"nics": [{"ipAddresses": [{`)

	if err := dockerClient.ContainerStart(context.Background(), c.ID, types.ContainerStartOptions{}); err != nil {
		t.Fatal(err)
	}

	handler := &StartHandler{Client: dockerClient,
		ContainerStateDir: stateDir}
	event := &events.Message{ID: c.ID}

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
	defer dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

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
	defer dockerClient.ContainerRemove(context.Background(), c.ID, types.ContainerRemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

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

func assertCheckCmdOutput(inputToCheck string, c types.ContainerJSON, dockerClient *client.Client, cmd []string, ifExists bool) (bool, error) {
	createExecConf := types.ExecConfig{
		AttachStdin:  false,
		AttachStdout: true,
		AttachStderr: false,
		Tty:          false,
		Cmd:          cmd,
	}
	createdExec, err := dockerClient.ContainerExecCreate(context.Background(), c.ID, createExecConf)
	if err != nil {
		return false, err
	}
	hijack, err := dockerClient.ContainerExecAttach(context.Background(), createdExec.ID, createExecConf)
	if err != nil {
		return false, err
	}

	lines := func(reader io.ReadCloser) chan string {
		lines := make(chan string)
		go func(r io.ReadCloser) {
			scanner := bufio.NewScanner(r)
			for scanner.Scan() {
				lines <- scanner.Text()
			}
			r.Close()
		}(reader)
		return lines
	}(hijack.Conn)

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
