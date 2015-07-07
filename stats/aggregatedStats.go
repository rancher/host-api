package stats

import (
	"encoding/json"
	"io"

	info "github.com/google/cadvisor/info/v1"
)

type AggregatedStats []AggregatedStat

type AggregatedStat struct {
	Id string `json:"id,omitempty"`
	*info.ContainerStats
}

func convertToAggregatedStats(stats []info.ContainerInfo, memLimit uint64) []AggregatedStats {
	maxDataPoints := len(stats[0].Stats)
	totalAggregatedStats := []AggregatedStats{}

	for i := 0; i < maxDataPoints; i++ {
		aggregatedStats := []AggregatedStat{}
		for _, stat := range stats {
			if len(stat.Stats) > i {
				statPoint := convertCadvisorStatToAggregatedStat(stat.Name, memLimit, stat.Stats[i])
				aggregatedStats = append(aggregatedStats, statPoint)
			}
		}
		totalAggregatedStats = append(totalAggregatedStats, aggregatedStats)
	}
	return totalAggregatedStats
}

func convertCadvisorStatToAggregatedStat(id string, memLimit uint64, stat *info.ContainerStats) AggregatedStat {
	aggregatedStat := AggregatedStat{}
	aggregatedStat.Id = id
	aggregatedStat.Timestamp = stat.Timestamp
	aggregatedStat.Cpu = stat.Cpu
	aggregatedStat.DiskIo = stat.DiskIo
	aggregatedStat.Memory = stat.Memory
	aggregatedStat.Network = stat.Network
	aggregatedStat.Filesystem = stat.Filesystem
	return aggregatedStat
}

func writeAggregatedStats(infos []info.ContainerInfo, memLimit uint64, writer io.Writer) error {
	aggregatedStats := convertToAggregatedStats(infos, memLimit)
	for _, stat := range aggregatedStats {
		data, err := json.Marshal(stat)
		if err != nil {
			return err
		}

		writer.Write(data)
		writer.Write([]byte("\n"))
	}

	return nil
}
