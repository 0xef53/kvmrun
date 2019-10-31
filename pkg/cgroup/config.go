// +build linux

package cgroup

// Config specifies parameters for the various subsystems.
type Config struct {
	// CPU shares (relative weight vs. other tasks)
	CpuShares int64 `json:"cpu_shares"`

	// CPU hardcap limit (in usecs). Allowed cpu time in a given period.
	CpuQuota int64 `json:"cpu_quota"`

	// CPU period to be used for hardcapping (in usecs).
	CpuPeriod int64 `json:"cpu_period"`

	// How many time CPU will use in realtime scheduling (in usecs).
	CpuRtRuntime int64 `json:"cpu_rt_runtime"`

	// CPU period to be used for realtime scheduling (in usecs).
	CpuRtPeriod int64 `json:"cpu_rt_period"`
}
