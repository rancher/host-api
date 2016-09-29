package cadvisor

import (
	"flag"
	"fmt"
	"github.com/golang/glog"
	"github.com/google/cadvisor/cache/memory"
	"github.com/google/cadvisor/container"
	"github.com/google/cadvisor/storage"
	"github.com/google/cadvisor/utils/sysfs"
	"time"
	"strings"
)

type metricSetValue struct {
	container.MetricSet
}

var (
	// Metrics to be ignored.
	// Tcp metrics are ignored by default.
	ignoreMetrics metricSetValue = metricSetValue{container.MetricSet{container.NetworkTcpUsageMetrics: struct{}{}}}

	// List of metrics that can be ignored.
	ignoreWhitelist = container.MetricSet{
		container.DiskUsageMetrics:       struct{}{},
		container.NetworkUsageMetrics:    struct{}{},
		container.NetworkTcpUsageMetrics: struct{}{},
	}
)

var (
	storageDriver   = flag.String("storage_driver", "", fmt.Sprintf("Storage `driver` to use. Data is always cached shortly in memory, this controls where data is pushed besides the local cache. Empty means none. Options are: <empty>, %s", strings.Join(storage.ListDrivers(), ", ")))
	storageDuration = flag.Duration("storage_duration", 2*time.Minute, "How long to keep data stored (Default: 2min).")
)

var MemoryLimits uint64 = 0

func GetCadvisorManager() (*Manager, error) {
	memoryStorage, err := NewMemoryStorage()
	if err != nil {
		return nil, err
	}
	sysFs, err := sysfs.NewRealSysFs()
	if err != nil {
		return nil, err
	}
	containerManager, err := New(memoryStorage, sysFs, 60*time.Second, true, ignoreMetrics.MetricSet, nil)
	return &containerManager, nil
}

// NewMemoryStorage creates a memory storage with an optional backend storage option.
func NewMemoryStorage() (*memory.InMemoryCache, error) {
	backendStorage, err := storage.New(*storageDriver)
	if err != nil {
		return nil, err
	}
	if *storageDriver != "" {
		glog.Infof("Using backend storage type %q", *storageDriver)
	}
	glog.Infof("Caching stats in memory for %v", *storageDuration)
	return memory.New(*storageDuration, backendStorage), nil
}

func StartUp() (error) {
	cadvisorManager, err := GetCadvisorManager()
	if err != nil {
		return err
	}
	if err := (*cadvisorManager).Start(); err != nil {
		return err
	}
	machineInfo, err := GetMachineInfo()
	if err != nil {
		return err
	}
	MemoryLimits = machineInfo.MemoryCapacity
	glog.Infof("Starting recovery of all containers")

	return nil
}
