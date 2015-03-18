package events

import (
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"github.com/go-fsnotify/fsnotify"
	"path"
)

func watchRancherState(eventChannel chan<- *docker.APIEvents) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Printf("event: %v", event)
				id := path.Base(event.Name)
				dockerEvent := &docker.APIEvents{
					ID:     id,
					Status: "start",
				}
				eventChannel <- dockerEvent
			case err := <-watcher.Errors:
				log.Printf("error: %v", err)
			}
		}
	}()

	err = watcher.Add(getContainerStateDir())
	if err != nil {
		log.Fatal(err)
	}

	<-done
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
