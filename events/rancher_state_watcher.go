package events

import (
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/fsouza/go-dockerclient"
	"gopkg.in/fsnotify.v1"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"time"
)

const healthCheckFileName = ".healthcheck"

type newWatcherFnDef func() (*fsnotify.Watcher, error)

type rancherStateWatcher struct {
	healthCheckTimeout       time.Duration
	healthCheckWriteInterval time.Duration
	restartWaitUnit          time.Duration
	maxRestarts              int
	watchDir                 string
	eventChannel             chan<- *docker.APIEvents
	watchInternal            func(chan<- *docker.APIEvents, string, time.Duration, time.Duration, newWatcherFnDef, <-chan bool) error
	stopChannel              <-chan bool
}

func newRancherStateWatcher(eventChannel chan<- *docker.APIEvents, watchDir string, stopChannel <-chan bool) *rancherStateWatcher {
	return &rancherStateWatcher{
		healthCheckTimeout:       time.Second * 10,
		healthCheckWriteInterval: time.Second * 8,
		restartWaitUnit:          time.Second,
		maxRestarts:              5,
		watchDir:                 watchDir,
		eventChannel:             eventChannel,
		watchInternal:            watchInternalFn,
		stopChannel:              stopChannel,
	}
}

func (w *rancherStateWatcher) watch() {
	// Ideally, I'd prefer to just bubble an error all the way up  with no restart logic
	// until it causes the process to exit (and be restared)t, but since this code currently
	// lives with the rest of host-api, I don't feel comfortable being so cavalier with process exits.
	// So this will try to restart a number of times, waiting expontentially
	restarts := 0
	restartWait := 0
	log.Infof("Watching state directory: %v", w.watchDir)
	for {
		if restarts >= w.maxRestarts {
			panic("Unable to successfully start rancher state watcher.")
		}
		if err := w.watchInternal(w.eventChannel, w.watchDir, w.healthCheckWriteInterval, w.healthCheckTimeout,
			newWatcherFn, w.stopChannel); err != nil {
			log.Warnf("Rancher state watcher returned with error. Waiting %v and then restarting. "+
				"Error that caused exit: %v", restartWait, err)
			time.Sleep(w.restartWaitUnit * time.Duration(restartWait))
			if restartWait == 0 {
				restartWait = 2
			} else {
				restartWait = restartWait * 2
			}
		} else {
			log.Infof("Exiting rancher state watcher. It returned without error.")
			return
		}
		restarts++
	}
}

func watchInternalFn(eventChannel chan<- *docker.APIEvents, watchDir string, healthCheckWriteInterval time.Duration,
	healthCheckTimeout time.Duration, newWatcher newWatcherFnDef, stopChannel <-chan bool) error {
	if watchDir == "" {
		// For backwards compatability, this shouldn't raise an error. Just log and return
		log.Info("Container state dir not configured. Returning without error.")
		return nil
	}

	if _, err := os.Stat(watchDir); err != nil {
		if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(watchDir, 0774); mkErr != nil {
				return fmt.Errorf("Unable to create state dir %v. Error: %v", watchDir, mkErr)
			}
		}
	}

	watcher, err := newWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	healthCheckChannel := make(chan fsnotify.Event, 1)
	watchErrorChannel := make(chan error, 1)

	go func() {
		defer close(healthCheckChannel)
		defer close(watchErrorChannel)
		for {
			select {
			case event := <-watcher.Events:
				log.Debugf("Received container state event: %v", event)
				wasntHealthCheck := !handleHealthCheck(event, healthCheckChannel)

				if wasntHealthCheck && event.Op == fsnotify.Create && event.Name != watchDir {
					id := path.Base(event.Name)
					if strings.HasPrefix(id, "tmp-") {
						break
					}
					dockerEvent := &docker.APIEvents{
						ID:     id,
						Status: "start",
						From:   "watcher-simulated",
					}
					eventChannel <- dockerEvent
				}
			case err := <-watcher.Errors:
				watchErrorChannel <- fmt.Errorf("Error event in container state dir watcher: %v", err)
			}
		}
	}()

	err = watcher.Add(watchDir)
	if err != nil {
		return fmt.Errorf("Error adding watcher for dir %v: %v", watchDir, err)
	}

	stopHealthCheckWrite := make(chan bool, 1)
	defer close(stopHealthCheckWrite)
	err = initHealthCheck(watchDir, healthCheckWriteInterval, stopHealthCheckWrite)
	if err != nil {
		return err
	}
	failedHealthCheck := make(chan bool, 1)
	go monitorHealth(healthCheckChannel, failedHealthCheck, healthCheckTimeout)

	select {
	case <-failedHealthCheck:
		return fmt.Errorf("Failed health check.")
	case watchErr := <-watchErrorChannel:
		return watchErr
	case <-stopChannel:
		return nil
	}
}

func newWatcherFn() (*fsnotify.Watcher, error) {
	return fsnotify.NewWatcher()
}

func writeHealthCheckFile(fileName string) error {
	if err := ioutil.WriteFile(fileName, []byte(""), 0644); err != nil {
		return fmt.Errorf("Couldn't create health check file. Error: %v", err)
	}
	return nil
}

func initHealthCheck(watchDir string, writeInterval time.Duration, stop <-chan bool) error {
	healthCheckFile := path.Join(watchDir, healthCheckFileName)

	if err := writeHealthCheckFile(healthCheckFile); err != nil {
		return err
	}

	go func() {
		ticker := time.NewTicker(writeInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := writeHealthCheckFile(healthCheckFile); err != nil {
					log.Warnf("Unable to write healthcheck file: %v", err)
				}
			case <-stop:
				return
			}
		}
	}()
	return nil
}

func monitorHealth(healthCheckChannel <-chan fsnotify.Event, failedCheck chan<- bool, timeout time.Duration) {
	for {
		timer := time.NewTimer(timeout)
		lastCheck := time.Now()
		select {
		case <-healthCheckChannel:
		case <-timer.C:
			now := time.Now()
			sinceLastCheck := now.Sub(lastCheck)
			if sinceLastCheck.Seconds() > timeout.Seconds() {
				failedCheck <- true
				return
			}
		}
	}
}

func handleHealthCheck(event fsnotify.Event, healthCheckChannel chan fsnotify.Event) bool {
	if strings.Contains(event.Name, healthCheckFileName) {
		healthCheckChannel <- event
		return true
	}
	return false
}
