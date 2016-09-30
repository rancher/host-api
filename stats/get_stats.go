package stats

import (
	"bufio"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

func getRootContainerInfo(count int) (containerInfo, error) {
	rootInfo := containerInfo{}
	rootStats := []*containerStats{}
	for i := 0; i < count; i++ {
		stats := containerStats{}
		// cpu
		cpuPerStats, err := cpu.Times(true)
		if err != nil {
			return containerInfo{}, err
		}
		cpuStats, err := cpu.Times(false)
		if err != nil {
			return containerInfo{}, err
		}
		stats.Cpu.Usage.PerCpu = []uint64{}
		for _, perStats := range cpuPerStats {
			stats.Cpu.Usage.PerCpu = append(stats.Cpu.Usage.PerCpu, uint64(perStats.User+perStats.System)*1000000000)
		}
		if len(cpuStats) > 0 {
			stats.Cpu.Usage.Total = uint64(cpuStats[0].User+cpuStats[0].System) * 1000000000
			stats.Cpu.Usage.User = uint64(cpuStats[0].User) * 1000000000
			stats.Cpu.Usage.System = uint64(cpuStats[0].System) * 1000000000
		}
		// memory
		memStats, err := mem.VirtualMemory()
		if err != nil {
			return containerInfo{}, err
		}
		stats.Memory.Usage = memStats.Used
		//disk
		diskIo, err := disk.IOCounters()
		if err != nil {
			return containerInfo{}, err
		}
		readBytes := uint64(0)
		writeBytes := uint64(0)
		for _, io := range diskIo {
			readBytes += io.ReadBytes
			writeBytes += io.WriteBytes
		}
		stats.DiskIo.IoServiceBytes = []PerDiskStats{}
		stats.DiskIo.IoServiceBytes = append(stats.DiskIo.IoServiceBytes, PerDiskStats{})
		stats.DiskIo.IoServiceBytes[0].Stats = map[string]uint64{}
		stats.DiskIo.IoServiceBytes[0].Stats["Read"] = readBytes
		stats.DiskIo.IoServiceBytes[0].Stats["Write"] = writeBytes
		//network
		netStats, err := net.IOCounters(false)
		if err != nil {
			return containerInfo{}, err
		}
		if len(netStats) > 0 {
			stats.Network.Name = netStats[0].Name
			stats.Network.RxBytes = netStats[0].BytesRecv
			stats.Network.TxBytes = netStats[0].BytesSent
		}
		rootStats = append(rootStats, &stats)
	}
	rootInfo.Stats = rootStats
	return rootInfo, nil
}

func getDockerContainerInfo(reader *bufio.Reader, count int, id string) (containerInfo, error) {
	return getContainerInfo(reader, count, id)
}

func getAllDockerContainers(readers []*bufio.Reader, count int, IDList []string) ([]containerInfo, error) {
	ret := []containerInfo{}
	for i, reader := range readers {
		contInfo, err := getContainerInfo(reader, count, IDList[i])
		if err != nil {
			return []containerInfo{}, err
		}
		ret = append(ret, contInfo)
	}
	return ret, nil
}
