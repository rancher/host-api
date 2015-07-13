package stats

import (
	"encoding/json"
	"io"

	info "github.com/google/cadvisor/info/v1"
)

type AggregatedStats []AggregatedStat

type AggregatedStat struct {
	Id           string `json:"id,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	*info.ContainerStats
}

func convertToAggregatedStats(id string, containerIds map[string]string, resourceType string, stats []info.ContainerInfo, memLimit uint64) []AggregatedStats {
	maxDataPoints := len(stats[0].Stats)
	totalAggregatedStats := []AggregatedStats{}

	for i := 0; i < maxDataPoints; i++ {
		aggregatedStats := []AggregatedStat{}
		for _, stat := range stats {
			if len(stat.Stats) > i {
				if resourceType == "container" && id == "" {
					id = stat.Name
				}
				statPoint := convertCadvisorStatToAggregatedStat(id, containerIds, resourceType, memLimit, stat.Stats[i])
				if statPoint.Id == "" {
					return totalAggregatedStats
				}
				aggregatedStats = append(aggregatedStats, statPoint)
			}
		}
		totalAggregatedStats = append(totalAggregatedStats, aggregatedStats)
	}
	return totalAggregatedStats
}

func convertCadvisorStatToAggregatedStat(id string, containerIds map[string]string, resourceType string, memLimit uint64, stat *info.ContainerStats) AggregatedStat {
	if resourceType == "container" {
		if id[:len("/docker/")] == "/docker/" {
			id = id[len("/docker/"):]
		}
		if idVal, ok := containerIds[id]; ok {
			id = idVal
		} else {
			return AggregatedStat{}
		}
	}
	return AggregatedStat{id, resourceType, stat}
}

func writeAggregatedStats(id string, containerIds map[string]string, resourceType string, infos []info.ContainerInfo, memLimit uint64, writer io.Writer) error {
	aggregatedStats := convertToAggregatedStats(id, containerIds, resourceType, infos, memLimit)
	for _, stat := range aggregatedStats {

		data, err := json.Marshal(stat)
		if err != nil {
			return err
		}

		_, err = writer.Write(data)
		if err != nil {
			return err
		}
		_, err = writer.Write([]byte("\n"))
		if err != nil {
			return err
		}
	}

	return nil
}
