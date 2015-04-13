package events

import (
	"github.com/fsouza/go-dockerclient"
	"os"
	"os/exec"
	"testing"
)

func useEnvVars() bool {
	return os.Getenv("CATTLE_DOCKER_USE_BOOT2DOCKER") == "true"
}

func createContainer(client *docker.Client) (*docker.Container, error) {
	opts := docker.CreateContainerOptions{Config: &docker.Config{Image: "tianon/true"}}
	return client.CreateContainer(opts)
}

func createNetTestContainer(client *docker.Client, ip string) (*docker.Container, error) {
	env := []string{"RANCHER_IP=" + ip}
	config := &docker.Config{
		Image:     "busybox:latest",
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
	client, _ := NewDockerClient(useEnvVars())
	pullTestImages(client)
	result := m.Run()
	os.Exit(result)
}

func prep(t *testing.T) *docker.Client {
	useEnvVars := useEnvVars()
	dockerClient, err := NewDockerClient(useEnvVars)
	if err != nil {
		t.Fatal(err)
	}

	if useEnvVars {
		buildCommand = func(pid string, ip string) *exec.Cmd {
			// Assumes boot2docker. Also assumes that:
			// - nsenter and net-utils.sh are on the path in the b2d vm
			// - This link exists: ln -s /proc/ /host/proc
			// See the README.md in this directory for setup details.
			return exec.Command("boot2docker", "ssh", "-t", "sudo", "net-util.sh", "-p", pid, "-i", ip)
		}
	}

	return dockerClient
}
