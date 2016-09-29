package stats

import (
	client "github.com/fsouza/go-dockerclient"
	"github.com/rancher/host-api/cadvisor"
	"gopkg.in/check.v1"
	"testing"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) {
	check.TestingT(t)
}

type ComputeTestSuite struct {
}

var _ = check.Suite(&ComputeTestSuite{})

func (s *ComputeTestSuite) SetUpSuite(c *check.C) {
}

func (s *ComputeTestSuite) TestRootContainerStats(c *check.C) {
	cadvisorManager, err := cadvisor.GetCadvisorManager()
	if err != nil {
		c.Fatal(err)
	}
	if err := (*cadvisorManager).Start(); err != nil {
		c.Fatal(err)
	}
	data, err := GetRootContainerInfo(30)
	if err != nil {
		c.Fatal(err)
	}
	stats := data.Stats
	c.Assert(stats[0].Cpu.Usage.Total, check.Not(check.Equals), 0)
}

func (s *ComputeTestSuite) TestDockerContainerStats(c *check.C) {
	cadvisorManager, err := cadvisor.GetCadvisorManager()
	if err != nil {
		c.Fatal(err)
	}
	if err := (*cadvisorManager).Start(); err != nil {
		c.Fatal(err)
	}
	createContainerOptions := client.CreateContainerOptions{
		Name: "statstest",
		Config: &client.Config{
			Image: "ibuildthecloud/helloworld:latest",
		},
		HostConfig: &client.HostConfig{
			Privileged: true,
		},
	}
	cli, err := client.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		c.Fatalf("Could not connect to docker, err: [%v]", err)
	}
	newCtr, err := cli.CreateContainer(createContainerOptions)
	if err != nil {
		c.Fatalf("Error creating container, err : [%v]", err)
	}
	err = cli.StartContainer(newCtr.ID, nil)
	if err != nil {
		c.Fatalf("Error starting container, err : [%v]", err)
	}
	defer func() {
		cli.StopContainer(newCtr.ID, 1)
		cli.RemoveContainer(client.RemoveContainerOptions{
			ID:            newCtr.ID,
			RemoveVolumes: true,
			Force:         true,
		})
	}()
	data, err := GetDockerContainerInfo("/docker/"+newCtr.ID, 30)
	if err != nil {
		c.Fatal(err)
	}
	stats := data.Stats
	c.Assert(stats[0].Cpu.Usage.Total, check.Not(check.Equals), 0)
}
