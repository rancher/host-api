package stats

import (
	"encoding/json"
	"io"
	"strings"

	info "github.com/google/cadvisor/info/v1"
)

type AggregatedStats []AggregatedStat

type AggregatedStat struct {
	Id           string `json:"id,omitempty"`
	ResourceType string `json:"resourceType,omitempty"`
	MemLimit     uint64 `json:"memLimit,omitempty"`
	*info.ContainerStats
}

func convertToAggregatedStats(id string, containerIds map[string]string, resourceType string, stats []info.ContainerInfo, memLimit uint64) []AggregatedStats {
	totalAggregatedStats := []AggregatedStats{}
	if len(stats) == 0 {
		return totalAggregatedStats
	}
	maxDataPoints := len(stats[0].Stats)

	for i := 0; i < maxDataPoints; i++ {
		aggregatedStats := []AggregatedStat{}
		for _, stat := range stats {
			if len(stat.Stats) > i {
				statPoint := convertCadvisorStatToAggregatedStat(id, stat.Aliases, stat.Name, containerIds, resourceType, memLimit, stat.Stats[i])
				if statPoint.Id == "" {
					continue
				}
				aggregatedStats = append(aggregatedStats, statPoint)
			}
		}
		totalAggregatedStats = append(totalAggregatedStats, aggregatedStats)
	}
	return totalAggregatedStats
}

func convertCadvisorStatToAggregatedStat(id string, aliases []string, name string, containerIds map[string]string, resourceType string, memLimit uint64, stat *info.ContainerStats) AggregatedStat {
	if resourceType == "container" {
		if id == "" {
			id = name
		}
		//Use last index because in case of running this nested inside a docker container, then the container Id of the containers becomes /docker/id-root/docker/id-of-container
		index := strings.LastIndex(id, "/docker/")
		if index != -1 {
			id = id[index+len("/docker/"):]
		}
		if idVal, ok := containerIds[id]; ok {
			id = idVal
		} else {
			found := false
			for _, alias := range aliases {
				if idVal, ok := containerIds[alias]; ok {
					id = idVal
					found = true
					break
				}
			}
			if !found {
				return AggregatedStat{}
			}
		}
	}
	return AggregatedStat{id, resourceType, memLimit, stat}
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
