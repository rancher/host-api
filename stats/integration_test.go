package stats

import (
	"flag"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"

	client "github.com/fsouza/go-dockerclient"

	"github.com/rancherio/host-api/config"
	"github.com/rancherio/host-api/test_utils"
	"github.com/rancherio/websocket-proxy/backend"
	"github.com/rancherio/websocket-proxy/proxy"
	wsp_utils "github.com/rancherio/websocket-proxy/test_utils"
)

var privateKey interface{}

func TestContainerStats(t *testing.T) {
	dialer := &websocket.Dialer{}
	headers := http.Header{}
	c, err := client.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("Could not connect to docker, err: [%v]", err)
	}

	createContainerOptions := client.CreateContainerOptions{
		Name: "cadvisortest",
		Config: &client.Config{
			Image: "google/cadvisor",
		},
	}

	newCtr, err := c.CreateContainer(createContainerOptions)
	if err != nil {
		t.Fatalf("Error creating container, err : [%v]", err)
	}
	err = c.StartContainer(newCtr.ID, nil)
	if err != nil {
		t.Fatalf("Error starting container, err : [%v]", err)
	}
	defer func() {
		c.StopContainer(newCtr.ID, 1)
		c.RemoveContainer(client.RemoveContainerOptions{
			ID:            newCtr.ID,
			RemoveVolumes: true,
			Force:         true,
		})
	}()
	newCtr, err = c.InspectContainer(newCtr.ID)
	if err != nil {
		t.Fatalf("Error inspecting container, err : [%v]", err)
	}
	log.Infof("%+v", newCtr)
	allCtrs, err := c.ListContainers(client.ListContainersOptions{})
	if err != nil {
		t.Fatalf("Error listing all images, err : [%v]", err)
	}
	ctrs := []client.APIContainers{}
	for _, ctr := range allCtrs {
		if strings.HasPrefix(ctr.Image, "google/cadvisor") {
			ctrs = append(ctrs, ctr)
		}
	}
	if len(ctrs) != 2 {
		t.Fatalf("Expected 2 containers, but got %v: [%v]", len(ctrs), ctrs)
	}

	cIds := map[string]string{}
	payload := map[string]interface{}{
		"hostUuid":     "1",
		"containerIds": cIds,
	}

	for i, ctr := range ctrs {
		cIds[ctr.ID] = "1i" + strconv.Itoa(i+1)
	}

	log.Infof("%+v", cIds)

	token := wsp_utils.CreateTokenWithPayload(payload, privateKey)
	url := "ws://localhost:1111/v1/containerstats?token=" + token
	ws, _, err := dialer.Dial(url, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	for count := 0; count < 4; count++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		stats := string(msg)
		if !strings.Contains(stats, "1i1") || !strings.Contains(stats, "1i2") {
			t.Fatalf("Stats are not working. Output: [%s]", stats)
		}
	}
}

// This test wont work in dind. Disabling it for now, until I figure out a solution
func unTestContainerStatSingleContainer(t *testing.T) {
	dialer := &websocket.Dialer{}
	headers := http.Header{}

	c, err := client.NewClient("unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("Could not connect to docker, err: [%v]", err)
	}

	ctrs, err := c.ListContainers(client.ListContainersOptions{
		Filters: map[string][]string{
			"image": []string{"google/cadvisor"},
		},
	})
	if err != nil || len(ctrs) == 0 {
		t.Fatalf("Error listing all images, err : [%v]", err)
	}
	payload := map[string]interface{}{
		"hostUuid": "1",
		"containerIds": map[string]string{
			ctrs[0].ID: "1i1",
		},
	}

	log.Info(ctrs[0].ID)

	token := wsp_utils.CreateTokenWithPayload(payload, privateKey)
	url := "ws://localhost:1111/v1/containerstats/" + ctrs[0].ID + "?token=" + token
	ws, _, err := dialer.Dial(url, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	for count := 0; count < 4; count++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		stats := string(msg)
		if !strings.Contains(stats, "1i1") {
			t.Fatalf("Stats are not working. Output: [%s]", stats)
		}
	}
}

func TestHostStats(t *testing.T) {
	dialer := &websocket.Dialer{}
	headers := http.Header{}

	payload := map[string]interface{}{
		"hostUuid":   "1",
		"resourceId": "1h1",
	}

	token := wsp_utils.CreateTokenWithPayload(payload, privateKey)
	url := "ws://localhost:1111/v1/hoststats?token=" + token
	ws, _, err := dialer.Dial(url, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	for count := 0; count < 4; count++ {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		stats := string(msg)
		if !strings.Contains(stats, "1h1") {
			t.Fatalf("Stats are not working. Output: [%s]", stats)
		}
	}
}

func TestHostStatsLegacy(t *testing.T) {
	dialer := &websocket.Dialer{}
	headers := http.Header{}
	token := wsp_utils.CreateToken("1", privateKey)
	url := "ws://localhost:1111/v1/stats?token=" + token
	ws, _, err := dialer.Dial(url, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()

	count := 0
	for {
		if count > 3 {
			break
		}

		_, msg, err := ws.ReadMessage()
		if err != nil {
			t.Fatal(err)
		}
		stats := string(msg)
		if !strings.Contains(stats, "cpu") {
			t.Fatalf("Stats are not working. Output: [%s]", stats)
		}
		count++
	}
}

func setupWebsocketProxy() {
	config.Parse()
	config.Config.NumStats = 1
	config.Config.CAdvisorUrl = "http://localhost:8080"
	config.Config.ParsedPublicKey = wsp_utils.ParseTestPublicKey()
	privateKey = wsp_utils.ParseTestPrivateKey()

	conf := test_utils.GetTestConfig(":1111")
	p := &proxy.ProxyStarter{
		BackendPaths:  []string{"/v1/connectbackend"},
		FrontendPaths: []string{"/v1/{logs:logs}/", "/v1/{stats:stats}", "/v1/{stats:stats}/{statsid}", "/v1/exec/"},
		StatsPaths: []string{"/v1/{hoststats:hoststats(\\/project)?(\\/)?}",
			"/v1/{containerstats:containerstats(\\/service)?(\\/)?}",
			"/v1/{containerstats:containerstats}/{containerid}"},
		Config: conf,
	}

	log.Infof("Starting websocket proxy. Listening on [%s], Proxying to cattle API at [%s].",
		conf.ListenAddr, conf.CattleAddr)

	go p.StartProxy()
	time.Sleep(time.Second)
	signedToken := wsp_utils.CreateBackendToken("1", privateKey)

	handlers := make(map[string]backend.Handler)
	handlers["/v1/stats/"] = &StatsHandler{}
	handlers["/v1/hoststats/"] = &HostStatsHandler{}
	handlers["/v1/containerstats/"] = &ContainerStatsHandler{}
	go backend.ConnectToProxy("ws://localhost:1111/v1/connectbackend?token="+signedToken, handlers)
	time.Sleep(300 * time.Millisecond)
}

func TestMain(m *testing.M) {
	flag.Parse()
	setupWebsocketProxy()
	os.Exit(m.Run())
}
