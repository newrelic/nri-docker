package docker

import (
	"context"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/newrelic/infra-integrations-sdk/data/metric"
	"github.com/newrelic/infra-integrations-sdk/integration"
	"github.com/newrelic/infra-integrations-sdk/log"
	"github.com/newrelic/nri-docker/src/stats"
)

const dockerClientVersion = "1.24" // todo: make configurable

type ContainerSampler struct {
	docker *client.Client
	stats  stats.Provider
}

func populate(ms *metric.Set, metrics []Metric) error {
	for _, metric := range metrics {
		if err := ms.SetMetric(metric.Name, metric.Value, metric.Type); err != nil {
			return err
		}
	}
	return nil
}

func attributeMetrics(container types.Container) []Metric {
	var cname string
	if len(container.Names) > 0 {
		cname = container.Names[0]
	}
	return []Metric{
		MetricCommandLine(container.Command),
		MetricContainerID(container.ID),
		MetricContainerName(cname),
		MetricContainerImage(container.ImageID),
		MetricContainerImageName(container.Image),
		MetricState(container.State),
		MetricStatus(container.Status),
	}
}

func (cs *ContainerSampler) statsMetrics(containerID string) []Metric {
	stats, err := cs.stats.Fetch(containerID)
	if err != nil {
		log.Error("error retrieving stats for container %s: %s", containerID, err.Error())
		return []Metric{}
	}

	cpu, mem, bio := stats.CPU(), stats.Memory(), stats.BlockingIO()
	return []Metric{
		MetricPIDs(float64(stats.PidsStats.Current)),
		MetricCPUPercent(cpu.CPU),
		MetricCPUKernelPercent(cpu.Kernel),
		MetricCPUUserPercent(cpu.User),
		MetricMemoryCacheBytes(mem.CacheUsageBytes),
		MetricMemoryUsageBytes(mem.UsageBytes),
		MetricMemoryResidentSizeBytes(mem.RSSUsageBytes),
		MetricMemorySizeLimitBytes(mem.MemLimitBytes),
		MetricIOTotalReadCount(bio.TotalReadCount),
		MetricIOTotalWriteCount(bio.TotalWriteCount),
		MetricIOTotalReadBytes(bio.TotalReadBytes),
		MetricIOTotalWriteBytes(bio.TotalWriteBytes),
		MetricIOReadCountPerSecond(bio.TotalReadCount),
		MetricIOWriteCountPerSecond(bio.TotalWriteCount),
		MetricIOReadBytesPerSecond(bio.TotalReadBytes),
		MetricIOWriteBytesPerSecond(bio.TotalWriteBytes),
	}
}

/*

func populateCPUStat(container docker.CgroupDockerStat, ms *metric.Set) error {


	rndCpu := 10 + rand.Float64()*10
	for _, metric := range []Metric{
		MetricContainerImage("12345"),
		MetricContainerImageName("alpine:latest"),
		MetricContainerName("containername"),
		MetricContainerID("123456"),
		MetricState("running"),
		{"label.docker.meta", metric.ATTRIBUTE, "label-value"},
		MetricCPUPercent(rndCpu),
		MetricCPUKernelPercent(rndCpu * 0.2),
		MetricCPUUserPercent(rndCpu * 0.8),
		MetricMemoryVirtualSizeBytes(10000000),
		MetricMemoryResidentSizeBytes(8000000),
		MetricIOReadCountPerSecond(0), // take from blkio_stats
		MetricIOWriteCountPerSecond(0),
		MetricIOReadBytesPerSecond(0),
		MetricIOWriteBytesPerSecond(0),
		MetricIOTotalReadCount(0),
		MetricIOTotalWriteCount(0),
		MetricIOTotalReadBytes(0),
		MetricIOTotalWriteBytes(0),
		MetricPIDs(1),
	} {
		if err := ms.SetMetric(metric.Name, metric.Value, metric.Type); err != nil {
			return err
		}
	}
	return nil
}

curl --unix-socket /var/run/docker.sock http:/docker/containers/<container_id>/stats
{
   "read":"2019-09-09T07:29:45.836354839Z",
   "preread":"0001-01-01T00:00:00Z",
   "pids_stats":{
      "current":2
   },
   "blkio_stats":{
      "io_service_bytes_recursive":[

      ],
      "io_serviced_recursive":[

      ],
      "io_queue_recursive":[

      ],
      "io_service_time_recursive":[

      ],
      "io_wait_time_recursive":[

      ],
      "io_merged_recursive":[

      ],
      "io_time_recursive":[

      ],
      "sectors_recursive":[

      ]
   },
   "num_procs":0,
   "storage_stats":{

   },
   "cpu_stats":{
      "cpu_usage":{
         "total_usage":42844380,
         "percpu_usage":[
            448537,
            4291148,
            5416364,
            32688331
         ],
         "usage_in_kernelmode":10000000,
         "usage_in_usermode":10000000
      },
      "system_cpu_usage":11071940000000,
      "online_cpus":4,
      "throttling_data":{
         "periods":0,
         "throttled_periods":0,
         "throttled_time":0
      }
   },
   "precpu_stats":{
      "cpu_usage":{
         "total_usage":0,
         "usage_in_kernelmode":0,
         "usage_in_usermode":0
      },
      "throttling_data":{
         "periods":0,
         "throttled_periods":0,
         "throttled_time":0
      }
   },
   "memory_stats":{
      "usage":1921024,
      "max_usage":2633728,
      "stats":{
         "active_anon":1449984,
         "active_file":0,
         "cache":12288,
         "dirty":0,
         "hierarchical_memory_limit":10485760,
         "hierarchical_memsw_limit":20971520,
         "inactive_anon":4096,
         "inactive_file":8192,
         "mapped_file":4096,
         "pgfault":1213,
         "pgmajfault":0,
         "pgpgin":883,
         "pgpgout":526,
         "rss":1449984,
         "rss_huge":0,
         "total_active_anon":1449984,
         "total_active_file":0,
         "total_cache":12288,
         "total_dirty":0,
         "total_inactive_anon":4096,
         "total_inactive_file":8192,
         "total_mapped_file":4096,
         "total_pgfault":1213,
         "total_pgmajfault":0,
         "total_pgpgin":883,
         "total_pgpgout":526,
         "total_rss":1449984,
         "total_rss_huge":0,
         "total_unevictable":0,
         "total_writeback":0,
         "unevictable":0,
         "writeback":0
      },
      "limit":10485760
   },
   "name":"/nginx",
   "id":"34923ff833ef87d498d493b54e8e7ae4d45a4ffc195a16b9dcd1a5e996a09639",
   "networks":{
      "eth0":{
         "rx_bytes":1248,
         "rx_packets":16,
         "rx_errors":0,
         "rx_dropped":0,
         "tx_bytes":0,
         "tx_packets":0,
         "tx_errors":0,
         "tx_dropped":0
      }
   }
}
*/
func NewContainerSampler(statsProvider stats.Provider) (ContainerSampler, error) {
	cli, err := client.NewEnvClient()
	cli.UpdateClientVersion(dockerClientVersion) // TODO: make it configurable
	return ContainerSampler{
		docker: cli,
		stats:  statsProvider,
	}, err
}

func (cs *ContainerSampler) SampleAll(entity *integration.Entity) error {
	// TODO: filter by state == running?
	containers, err := cs.docker.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		return err
	}
	for _, container := range containers {
		ms := entity.NewMetricSet(ContainerSampleName,
			metric.Attr("removeme", container.ID)) // TODO: provide other unique label

		if err := populate(ms, attributeMetrics(container)); err != nil {
			return err
		}

		if err := populate(ms, cs.statsMetrics(container.ID)); err != nil {
			return err
		}

		cs.statsMetrics(container.ID)
	}
	return nil
}
