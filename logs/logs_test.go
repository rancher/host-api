package logs

import (
	"net/http"
	// "strconv"
	"io"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/gorilla/websocket"
	"gopkg.in/check.v1"

	"github.com/rancherio/websocket-proxy/backend"
	"github.com/rancherio/websocket-proxy/proxy"
	wsp_utils "github.com/rancherio/websocket-proxy/test_utils"

	"github.com/rancherio/host-api/config"
	"github.com/rancherio/host-api/events"
	"github.com/rancherio/host-api/test_utils"
)

var privateKey interface{}

func Test(t *testing.T) {
	check.TestingT(t)
}

type LogsTestSuite struct {
	client *docker.Client
}

var _ = check.Suite(&LogsTestSuite{})

func (s *LogsTestSuite) TestLogs(c *check.C) {
	dialer := &websocket.Dialer{}
	headers := http.Header{}

	createContainerOptions := docker.CreateContainerOptions{
		Name: "logstest",
		Config: &docker.Config{
			Image:     "hello-world",
			OpenStdin: true,
			Tty:       true,
		},
	}

	newCtr, err := s.client.CreateContainer(createContainerOptions)
	if err != nil {
		c.Fatalf("Error creating container, err : [%v]", err)
	}
	err = s.client.StartContainer(newCtr.ID, nil)
	if err != nil {
		c.Fatalf("Error starting container, err : [%v]", err)
	}
	defer func() {
		s.client.StopContainer(newCtr.ID, 1)
		s.client.RemoveContainer(docker.RemoveContainerOptions{
			ID:            newCtr.ID,
			RemoveVolumes: true,
			Force:         true,
		})
	}()
	newCtr, err = s.client.InspectContainer(newCtr.ID)
	if err != nil {
		c.Fatalf("Error inspecting container, err : [%v]", err)
	}
	log.Infof("Log test container: %+v", newCtr)

	payload := map[string]interface{}{
		"hostUuid": "1",
		"logs": map[string]interface{}{
			"Container": newCtr.ID,
			"Follow":    true,
		},
	}

	token := wsp_utils.CreateTokenWithPayload(payload, privateKey)
	url := "ws://localhost:3333/v1/logs/?token=" + token
	ws, _, err := dialer.Dial(url, headers)
	if err != nil {
		c.Fatal(err)
	}
	defer ws.Close()

	for count := 0; count < 20; count++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			if err == io.EOF {
				return
			}
			c.Fatal(err)
		}
		msgStr := string(msg)
		log.Infof("MESSAGE %v: |%s|", count, msgStr)
		if !strings.HasPrefix(msgStr, "00") {
			//		c.Fatalf("Message didn't have 00 prefix: [%s]", msgStr)
		}
	}
}

func (s *LogsTestSuite) setupWebsocketProxy() {
	config.Parse()
	config.Config.HostUuid = "1"
	config.Config.ParsedPublicKey = wsp_utils.ParseTestPublicKey()
	privateKey = wsp_utils.ParseTestPrivateKey()

	conf := test_utils.GetTestConfig(":3333")
	p := &proxy.ProxyStarter{
		BackendPaths:  []string{"/v1/connectbackend"},
		FrontendPaths: []string{"/v1/{logs:logs}/"},
		Config:        conf,
	}

	log.Infof("Starting websocket proxy. Listening on [%s], Proxying to cattle API at [%s].",
		conf.ListenAddr, conf.CattleAddr)

	go p.StartProxy()
	time.Sleep(time.Second)
	signedToken := wsp_utils.CreateBackendToken("1", privateKey)

	handlers := make(map[string]backend.Handler)
	handlers["/v1/logs/"] = &LogsHandler{}
	go backend.ConnectToProxy("ws://localhost:3333/v1/connectbackend?token="+signedToken, handlers)
	s.pullImage("hello-world", "latest")
}

func (s *LogsTestSuite) SetUpSuite(c *check.C) {
	cli, err := events.NewDockerClient()
	if err != nil {
		c.Fatalf("Could not connect to docker, err: [%v]", err)
	}
	s.client = cli
	s.setupWebsocketProxy()
}

func (s *LogsTestSuite) pullImage(imageRepo, imageTag string) error {
	imageOptions := docker.PullImageOptions{
		Repository: imageRepo,
		Tag:        imageTag,
	}
	imageAuth := docker.AuthConfiguration{}
	log.Infof("Pulling %v:%v image.", imageRepo, imageTag)
	return s.client.PullImage(imageOptions, imageAuth)
}
