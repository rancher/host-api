package events

import (
	"os"
	"testing"

	"github.com/fsouza/go-dockerclient"
)

func useEnvVars() bool {
	return os.Getenv("CATTLE_DOCKER_USE_BOOT2DOCKER") == "true"
}

func createContainer(client *docker.Client) (*docker.Container, error) {
	opts := docker.CreateContainerOptions{Config: &docker.Config{Image: "tianon/true"}}
	return client.CreateContainer(opts)
}

func createNetTestContainerNoLabel(client *docker.Client, ip string) (*docker.Container, error) {
	return createTestContainerInternal(client, ip, false, nil, true)
}

func createNetTestContainer(client *docker.Client, ip string) (*docker.Container, error) {
	return createTestContainerInternal(client, ip, true, nil, true)
}

func createTestContainer(client *docker.Client, ip string, labels map[string]string, isSystem bool) (*docker.Container, error) {
	return createTestContainerInternal(client, ip, true, labels, isSystem)
}

func createTestContainerInternal(client *docker.Client, ip string, useLabel bool, inputLabels map[string]string, isSystem bool) (*docker.Container, error) {
	labels := make(map[string]string)
	if inputLabels != nil {
		for k, v := range inputLabels {
			labels[k] = v
		}
	}
	if isSystem {
		labels["io.rancher.container.system"] = "FakeSysContainer"
	}

	env := []string{}
	if ip != "" {
		if useLabel {
			labels["io.rancher.container.ip"] = ip
		} else {
			env = append(env, "RANCHER_IP="+ip)
		}
	}

	config := &docker.Config{
		Image:     "busybox:latest",
		Labels:    labels,
		Env:       env,
		OpenStdin: true,
		StdinOnce: false,
	}
	opts := docker.CreateContainerOptions{Config: config}
	return client.CreateContainer(opts)
}

func pullTestImages(client *docker.Client) {
	listImageOpts := docker.ListImagesOptions{}
	images, _ := client.ListImages(listImageOpts)
	imageMap := map[string]bool{}
	for _, image := range images {
		for _, tag := range image.RepoTags {
			imageMap[tag] = true
		}
	}

	var pullImage = func(repo string) {
		if _, ok := imageMap[repo]; !ok {
			imageOptions := docker.PullImageOptions{
				Repository: repo,
			}
			imageAuth := docker.AuthConfiguration{}
			client.PullImage(imageOptions, imageAuth)
		}
	}

	imageName := "tianon/true:latest"
	pullImage(imageName)

	imageName = "busybox:latest"
	pullImage(imageName)
}

func TestMain(m *testing.M) {
	client, _ := NewDockerClient()
	pullTestImages(client)
	result := m.Run()
	os.Exit(result)
}

func prep(t *testing.T) *docker.Client {
	dockerClient, err := NewDockerClient()
	if err != nil {
		t.Fatal(err)
	}

	return dockerClient
}
