package stats

import (
	"flag"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/gorilla/websocket"

	"github.com/rancherio/host-api/config"
	"github.com/rancherio/host-api/test_utils"
	"github.com/rancherio/websocket-proxy/backend"
	"github.com/rancherio/websocket-proxy/proxy"
	wsp_utils "github.com/rancherio/websocket-proxy/test_utils"
)

var privateKey interface{}

func TestHostStats(t *testing.T) {
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
	config.Config.CAdvisorUrl = "http://localhost:8080"
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
