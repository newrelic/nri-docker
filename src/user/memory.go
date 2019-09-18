package user

import "github.com/containerd/cgroups"

type Memory struct {
	UsageBytes      uint64
	CacheUsageBytes uint64
	RSSUsageBytes   uint64
	MemLimitBytes   uint64
}

func MemoryMetrics(metric *cgroups.Metrics) Memory {



	mem := Memory{}
	if metric.Memory == nil {
		return mem
	}
	if metric.Memory.Usage != nil {
		mem.MemLimitBytes = metric.Memory.Usage.Limit
	}
	mem.CacheUsageBytes = metric.Memory.Cache
	mem.RSSUsageBytes = metric.Memory.RSS

	/* Calculating usage instead of `memory.usage_in_bytes` file contents.
	https://www.kernel.org/doc/Documentation/cgroup-v1/memory.txt
	For efficiency, as other kernel components, memory cgroup uses some optimization
	to avoid unnecessary cacheline false sharing. usage_in_bytes is affected by the
	method and doesn't show 'exact' value of memory (and swap) usage, it's a fuzz
	value for efficient access. (Of course, when necessary, it's synchronized.)
	If you want to know more exact memory usage, you should use RSS+CACHE(+SWAP)
	value in memory.stat(see 5.2).
	However, as the `docker stats` cli tool does, page cache is intentionally
	excluded to avoid misinterpretation of the output.
	Also the Swap usage is parsed from memory.memsw.usage_in_bytes, which
	according to the documentation reports the sum of current memory usage
	plus swap space used by processes in the cgroup (in bytes). That's why
	Usage is subtracted from the Swap: to get the actual swap.
	*/
	mem.UsageBytes = metric.Memory.RSS
	if metric.Memory.Swap != nil {
		mem.UsageBytes += metric.Memory.Swap.Usage - metric.Memory.Usage.Usage
	}

	return mem
}
