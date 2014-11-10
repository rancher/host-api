package config

import (
	"flag"
	"github.com/rakyll/globalconf"
	"os"
)

type config struct {
	CAdvisorUrl string
	DockerUrl   string
	Systemd     bool
	NumStats    int
	Auth        bool
	Key         string
	HostUuid    string
	Port        int
	Ip          string
}

var Config config

func Parse() error {
	flag.IntVar(&Config.Port, "port", 8080, "Listen port")
	flag.StringVar(&Config.Ip, "ip", "", "Listen IP, defaults to all IPs")
	flag.StringVar(&Config.CAdvisorUrl, "cadvisor-url", "http://localhost:8081", "cAdvisor URL")
	flag.StringVar(&Config.DockerUrl, "docker-host", "unix:///var/run/docker.sock", "Docker host URL")
	flag.IntVar(&Config.NumStats, "num-stats", 600, "Number of stats to show by default")
	flag.BoolVar(&Config.Auth, "auth", false, "Authenticate requests")
	flag.StringVar(&Config.HostUuid, "host-uuid", "", "Host UUID")
	flag.StringVar(&Config.Key, "public-key", "", "Public Key for Authentication")

	confOptions := &globalconf.Options{
		EnvPrefix: "HOST_API_",
	}

	filename := os.Getenv("HOST_API_CONFIG_FILE")
	if len(filename) > 0 {
		confOptions.Filename = filename
	}

	conf, err := globalconf.NewWithOptions(confOptions)

	if err != nil {
		return err
	}

	conf.ParseAll()

	s, err := os.Stat("/run/systemd/system")
	if err != nil || !s.IsDir() {
		Config.Systemd = false
	} else {
		Config.Systemd = true
	}

	return nil
}
