package stats

import (
	"fmt"
	"strings"

	"bufio"
	"encoding/json"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/shirou/gopsutil/mem"
	"time"
)

func pathParts(path string) []string {
	path = strings.TrimPrefix(path, "/")
	path = strings.TrimSuffix(path, "/")
	return strings.Split(path, "/")
}

func parseRequestToken(tokenString string, parsedPublicKey interface{}) (*jwt.Token, error) {
	if tokenString == "" {
		return nil, fmt.Errorf("No JWT token provided")
	}

	return jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return parsedPublicKey, nil
	})
}

func getContainerStats(reader *bufio.Reader, count int, id string) (containerInfo, error) {
	i, err := getDockerContainerInfo(reader, count, id)
	return i, err
}

type DockerStats struct {
	Read      time.Time `json:"read"`
	PidsStats struct {
		Current int `json:"current"`
	} `json:"pids_stats"`
	Networks struct {
		NetworkInterface map[string]struct {
			RxBytes   int `json:"rx_bytes"`
			RxDropped int `json:"rx_dropped"`
			RxErrors  int `json:"rx_errors"`
			RxPackets int `json:"rx_packets"`
			TxBytes   int `json:"tx_bytes"`
			TxDropped int `json:"tx_dropped"`
			TxErrors  int `json:"tx_errors"`
			TxPackets int `json:"tx_packets"`
		}
	} `json:"networks"`
	BlkioStats struct {
		IoServiceBytesRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"io_service_bytes_recursive"`
		IoServicedRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"io_serviced_recursive"`
		IoQueueRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"io_queue_recursive"`
		IoServiceTimeRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"io_service_time_recursive"`
		IoWaitTimeRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"io_wait_time_recursive"`
		IoMergedRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"io_merged_recursive"`
		IoTimeRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"io_time_recursive"`
		SectorsRecursive []struct {
			Major int    `json:"major"`
			Minor int    `json:"minor"`
			Op    string `json:"op"`
			Value int    `json:"value"`
		} `json:"sectors_recursive"`
	} `json:"blkio_stats"`
	MemoryStats struct {
		Stats struct {
			TotalPgmajfault         int `json:"total_pgmajfault"`
			Cache                   int `json:"cache"`
			MappedFile              int `json:"mapped_file"`
			TotalInactiveFile       int `json:"total_inactive_file"`
			Pgpgout                 int `json:"pgpgout"`
			Rss                     int `json:"rss"`
			TotalMappedFile         int `json:"total_mapped_file"`
			Writeback               int `json:"writeback"`
			Unevictable             int `json:"unevictable"`
			Pgpgin                  int `json:"pgpgin"`
			TotalUnevictable        int `json:"total_unevictable"`
			Pgmajfault              int `json:"pgmajfault"`
			TotalRss                int `json:"total_rss"`
			TotalRssHuge            int `json:"total_rss_huge"`
			TotalWriteback          int `json:"total_writeback"`
			TotalInactiveAnon       int `json:"total_inactive_anon"`
			RssHuge                 int `json:"rss_huge"`
			HierarchicalMemoryLimit int `json:"hierarchical_memory_limit"`
			TotalPgfault            int `json:"total_pgfault"`
			TotalActiveFile         int `json:"total_active_file"`
			ActiveAnon              int `json:"active_anon"`
			TotalActiveAnon         int `json:"total_active_anon"`
			TotalPgpgout            int `json:"total_pgpgout"`
			TotalCache              int `json:"total_cache"`
			InactiveAnon            int `json:"inactive_anon"`
			ActiveFile              int `json:"active_file"`
			Pgfault                 int `json:"pgfault"`
			InactiveFile            int `json:"inactive_file"`
			TotalPgpgin             int `json:"total_pgpgin"`
		} `json:"stats"`
		MaxUsage int `json:"max_usage"`
		Usage    int `json:"usage"`
		Failcnt  int `json:"failcnt"`
		Limit    int `json:"limit"`
	} `json:"memory_stats"`
	CPUStats struct {
		CPUUsage struct {
			PercpuUsage       []int `json:"percpu_usage"`
			UsageInUsermode   int   `json:"usage_in_usermode"`
			TotalUsage        int   `json:"total_usage"`
			UsageInKernelmode int   `json:"usage_in_kernelmode"`
		} `json:"cpu_usage"`
		SystemCPUUsage int64 `json:"system_cpu_usage"`
		ThrottlingData struct {
			Periods          int `json:"periods"`
			ThrottledPeriods int `json:"throttled_periods"`
			ThrottledTime    int `json:"throttled_time"`
		} `json:"throttling_data"`
	} `json:"cpu_stats"`
	PrecpuStats struct {
		CPUUsage struct {
			PercpuUsage       []int `json:"percpu_usage"`
			UsageInUsermode   int   `json:"usage_in_usermode"`
			TotalUsage        int   `json:"total_usage"`
			UsageInKernelmode int   `json:"usage_in_kernelmode"`
		} `json:"cpu_usage"`
		SystemCPUUsage int64 `json:"system_cpu_usage"`
		ThrottlingData struct {
			Periods          int `json:"periods"`
			ThrottledPeriods int `json:"throttled_periods"`
			ThrottledTime    int `json:"throttled_time"`
		} `json:"throttling_data"`
	} `json:"precpu_stats"`
}

type containerInfo struct {
	Id    string
	Stats []*containerStats
}

type containerStats struct {
	Timestamp time.Time    `json:"timestamp"`
	Cpu       CpuStats     `json:"cpu,omitempty"`
	DiskIo    DiskIoStats  `json:"diskio,omitempty"`
	Network   NetworkStats `json:"network,omitempty"`
	Memory    MemoryStats  `json:"memory,omitempty"`
}

type CpuStats struct {
	Usage CpuUsage `json:"usage"`
}

type CpuUsage struct {
	// Total CPU usage.
	// Units: nanoseconds
	Total uint64 `json:"total"`

	// Per CPU/core usage of the container.
	// Unit: nanoseconds.
	PerCpu []uint64 `json:"per_cpu_usage,omitempty"`

	// Time spent in user space.
	// Unit: nanoseconds
	User uint64 `json:"user"`

	// Time spent in kernel space.
	// Unit: nanoseconds
	System uint64 `json:"system"`
}

type DiskIoStats struct {
	IoServiceBytes []PerDiskStats `json:"io_service_bytes,omitempty"`
}

type PerDiskStats struct {
	Major uint64            `json:"major"`
	Minor uint64            `json:"minor"`
	Stats map[string]uint64 `json:"stats"`
}

type NetworkStats struct {
	InterfaceStats
	Interfaces []InterfaceStats `json:"interfaces,omitempty"`
}

type MemoryStats struct {
	// Current memory usage, this includes all memory regardless of when it was
	// accessed.
	// Units: Bytes.
	Usage uint64 `json:"usage"`
}

type InterfaceStats struct {
	// The name of the interface.
	Name string `json:"name"`
	// Cumulative count of bytes received.
	RxBytes uint64 `json:"rx_bytes"`
	// Cumulative count of packets received.
	RxPackets uint64 `json:"rx_packets"`
	// Cumulative count of receive errors encountered.
	RxErrors uint64 `json:"rx_errors"`
	// Cumulative count of packets dropped while receiving.
	RxDropped uint64 `json:"rx_dropped"`
	// Cumulative count of bytes transmitted.
	TxBytes uint64 `json:"tx_bytes"`
	// Cumulative count of packets transmitted.
	TxPackets uint64 `json:"tx_packets"`
	// Cumulative count of transmit errors encountered.
	TxErrors uint64 `json:"tx_errors"`
	// Cumulative count of packets dropped while transmitting.
	TxDropped uint64 `json:"tx_dropped"`
}

func getContainerInfo(reader *bufio.Reader, count int, id string) (containerInfo, error) {
	contInfo := containerInfo{}
	contInfo.Id = id
	stats := []*containerStats{}
	for i := 0; i < count; i++ {
		str, err := reader.ReadString([]byte("\n")[0])
		if err != nil {
			return containerInfo{}, err
		}
		dockerStats, err := FromString(str)
		if err != nil {
			return containerInfo{}, err
		}
		contStats := convertDockerStats(dockerStats)
		stats = append(stats, contStats)
	}
	contInfo.Stats = stats
	return contInfo, nil
}

func convertDockerStats(stats DockerStats) *containerStats {
	containerStats := containerStats{}
	containerStats.Timestamp = stats.Read
	containerStats.Cpu.Usage.Total = uint64(stats.CPUStats.CPUUsage.TotalUsage)
	containerStats.Cpu.Usage.PerCpu = []uint64{}
	for _, value := range stats.CPUStats.CPUUsage.PercpuUsage {
		containerStats.Cpu.Usage.PerCpu = append(containerStats.Cpu.Usage.PerCpu, uint64(value))
	}
	containerStats.Cpu.Usage.System = uint64(stats.CPUStats.CPUUsage.UsageInKernelmode)
	containerStats.Cpu.Usage.User = uint64(stats.CPUStats.CPUUsage.UsageInKernelmode)
	containerStats.Memory.Usage = uint64(stats.MemoryStats.Usage)
	containerStats.Network.Interfaces = []InterfaceStats{}
	for name, netStats := range stats.Networks.NetworkInterface {
		data := InterfaceStats{}
		data.Name = name
		data.RxBytes = uint64(netStats.RxBytes)
		data.RxDropped = uint64(netStats.RxDropped)
		data.RxErrors = uint64(netStats.RxErrors)
		data.RxPackets = uint64(netStats.RxPackets)
		data.TxBytes = uint64(netStats.TxBytes)
		data.TxDropped = uint64(netStats.TxDropped)
		data.TxPackets = uint64(netStats.TxPackets)
		data.TxErrors = uint64(netStats.TxErrors)
		containerStats.Network.Interfaces = append(containerStats.Network.Interfaces, data)
	}
	containerStats.DiskIo.IoServiceBytes = []PerDiskStats{}
	for _, diskStats := range stats.BlkioStats.IoServiceBytesRecursive {
		data := PerDiskStats{}
		data.Stats = map[string]uint64{}
		data.Stats[diskStats.Op] = uint64(diskStats.Value)
		containerStats.DiskIo.IoServiceBytes = append(containerStats.DiskIo.IoServiceBytes, data)
	}
	return &containerStats
}

func FromString(rawstring string) (DockerStats, error) {
	obj := DockerStats{}
	err := json.Unmarshal([]byte(rawstring), &obj)
	if err != nil {
		return obj, err
	}
	return obj, nil
}

func getMemCapcity() (uint64, error) {
	data, err := mem.VirtualMemory()
	if err != nil {
		return 0, err
	}
	return data.Total, nil
}
