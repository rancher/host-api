package events

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/go-fsnotify/fsnotify"
	"os"
	"path"
)

func watchRancherState(eventChannel chan<- *docker.APIEvents) error {
	watchDir := getContainerStateDir()
	if watchDir == "" {
		log.Info("Container state dir not configured.")
		return nil
	}
	if _, err := os.Stat(watchDir); err != nil {
		return fmt.Errorf("Couldn't determine if container state dir exists. Error: %v", err)
	}

	// TODO FIGURE OUT IF WE NEED/WANT TO CLOSE WATCHER
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Debugf("Received container state event: %v", event)
				if event.Op == fsnotify.Create && len(event.Name) > len(watchDir) {
					id := path.Base(event.Name)
					dockerEvent := &docker.APIEvents{
						ID:     id,
						Status: "start",
					}
					eventChannel <- dockerEvent
				}
			case err := <-watcher.Errors:
				log.Errorf("Error event in container state dir watcher: %v", err)
			}
		}
	}()

	err = watcher.Add(watchDir)
	if err != nil {
		return fmt.Errorf("Error adding watcher to dir %v: %v", watchDir, err)
	}

	return nil
}

type subnet struct {
	CidrSize int
}

type ipAddress struct {
	Address string
	Subnet  subnet
	Role    string
}

type nic struct {
	IpAddresses []ipAddress `json:"ipAddresses"`
	Id          string
}

type instance struct {
	Nics []nic
}
