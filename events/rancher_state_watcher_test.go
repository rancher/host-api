package events

import (
	"fmt"
	"github.com/fsouza/go-dockerclient"
	"gopkg.in/fsnotify.v1"
	"io/ioutil"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"
)

const watchTestDir = "watch-test"

func TestNoWatchDir(t *testing.T) {
	err := watchInternalFn(make(chan *docker.APIEvents, 1), "", time.Millisecond*1, time.Millisecond*1,
		newWatcherFn, make(chan bool, 1))
	if err != nil {
		t.FailNow()
	}
}

func TestWatchDirHappyPath(t *testing.T) {
	purgeDir(t)
	mkTestDir(t)
	eventChan, stopChan := initWatcher(t, nil)
	defer close(stopChan)
	fileName := makeTestEventFile(t)
	assertEvent(fileName, eventChan, t)
}

func TestDirectoryDoesntExist(t *testing.T) {
	// Note that we're starting the watcher without creating the directory.
	purgeDir(t)
	eventChan, stopChan := initWatcher(t, nil)
	defer close(stopChan)
	fileName := makeTestEventFile(t)
	assertEvent(fileName, eventChan, t)
}

func TestDirectoryGetsDeleted(t *testing.T) {
	// Directory gets deleted after the watcher starts. Expected behavior is for that
	// deletion to be picked up by the health check logic and cause the watcher function
	// to exit and restart.
	purgeDir(t)

	// default healthcheck wait seconds. Tune it down to milliseconds for fast checking
	healthCheckTimeout := time.Millisecond * 100
	healthCheckWriteInterval := time.Millisecond * 80
	var postInit = func(watcher *rancherStateWatcher) {
		watcher.healthCheckTimeout = healthCheckTimeout
		watcher.healthCheckWriteInterval = healthCheckWriteInterval
	}
	eventChan, stopChan := initWatcher(t, postInit)
	defer close(stopChan)
	purgeDir(t)

	// give the health check enough time to discover the problem
	time.Sleep(time.Millisecond * 250)
	fileName := makeTestEventFile(t)
	assertEvent(fileName, eventChan, t)
}

func TestDirAndFilesAlreadyExists(t *testing.T) {
	// Just ensures that a pre-existing dir and file(s) doesn't interfere with any logic.
	purgeDir(t)
	mkTestDir(t)
	makeTestEventFile(t)
	eventChan, stopChan := initWatcher(t, nil)
	defer close(stopChan)
	fileName := makeTestEventFile(t)
	assertEvent(fileName, eventChan, t)
}

func TestRestartLogic(t *testing.T) {
	eventChan := make(chan *docker.APIEvents, 10)
	stopChan := make(chan bool, 1)
	defer close(stopChan)
	watcher := newRancherStateWatcher(eventChan, watchTestDir, stopChan)
	watcher.restartWaitUnit = time.Millisecond
	watcher.maxRestarts = 5
	restartCount := 0

	var mockWatchInternal = func(eventChannel chan<- *docker.APIEvents, watchDir string, interval,
		timeout time.Duration, newWatcherFn newWatcherFnDef, stopChannel <-chan bool) error {
		restartCount++
		return fmt.Errorf("Test error %v", restartCount)
	}
	watcher.watchInternal = mockWatchInternal

	paniced := false
	func() {
		defer func() {
			if err := recover(); err != nil {
				paniced = true
			}
		}()

		watcher.watch()
	}()

	if !paniced {
		t.Fatalf("Didn't panic when expected.")
	}

	if restartCount != watcher.maxRestarts {
		t.Fatalf("Unexpected number of restart attempts: %v", restartCount)
	}
}

func TestFSNotifyErrorChannel(t *testing.T) {
	// Proves that watchInternalFn will return an error when the watcher sends
	// and error over the Error channel.
	errors := make(chan error, 1)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	watcher.Errors = errors
	mockNewWatcher := func() (*fsnotify.Watcher, error) {
		return watcher, nil
	}
	errors <- fmt.Errorf("Fake error")

	eventChan := make(chan *docker.APIEvents, 10)
	stopChan := make(chan bool, 1)
	defer close(stopChan)
	if err := watchInternalFn(eventChan, watchTestDir, time.Millisecond*1,
		time.Millisecond*1, mockNewWatcher, stopChan); err == nil {
		t.Fatalf("Expected to get an error.")
	}
}

func assertEvent(fileName string, eventChan chan *docker.APIEvents, t *testing.T) {
	select {
	case event := <-eventChan:
		if event.ID != fileName || event.Status != "start" ||
			event.From != simulatedEvent || event.Time != 0 {
			t.Fatalf("Unexpected event: %#v", event)
		}
	case <-time.NewTimer(time.Millisecond * 300).C:
		t.Fatalf("Timed out waiting for event")
	}
}

func initWatcher(t *testing.T, postInit func(*rancherStateWatcher)) (chan *docker.APIEvents, chan bool) {
	eventChan := make(chan *docker.APIEvents, 10)
	stopChan := make(chan bool, 1)
	watcher := newRancherStateWatcher(eventChan, watchTestDir, stopChan)

	if postInit != nil {
		postInit(watcher)
	}

	go watcher.watch()
	// This sleep is a cheat to let the watch initialize. If tests ever fail because of it,
	// will need to rework into a "ready" channel
	time.Sleep(time.Millisecond * 10)
	return eventChan, stopChan
}

func makeTestEventFile(t *testing.T) string {
	id := randomString()
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	filePath := path.Join(currentDir, watchTestDir, id)
	tmpFilePath := path.Join(currentDir, watchTestDir, "tmp-"+id)
	if err := ioutil.WriteFile(tmpFilePath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	// We write to a temp file and then move to mimic how the file is written in production.
	// This is done to achieve an atomic write operation.
	if err := os.Rename(tmpFilePath, filePath); err != nil {
		t.Fatal(err)
	}
	return id
}

func mkTestDir(t *testing.T) {
	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(path.Join(currentDir, watchTestDir), 0777); err != nil {
		t.Fatal(err)
	}
}

func purgeDir(t *testing.T) {
	if err := os.RemoveAll(watchTestDir); err != nil {
		t.Logf("Didn't purge test dir. Reason: %v", err)
	}
}

var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func randomString() string {
	length := 10
	b := make([]rune, length)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
