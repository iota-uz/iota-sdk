package viewmodels

import (
	"net/http"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	sdkhealth "github.com/iota-uz/iota-sdk/pkg/health"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type SystemInfoMetrics struct {
	GoVersion           string
	GOOS                string
	GOARCH              string
	NumCPU              int
	NumGoroutines       int
	HeapAlloc           uint64
	HeapSys             uint64
	Uptime              time.Duration
	CPUPercent          float64
	MemoryPercent       float64
	DiskPercent         float64
	TotalMemory         uint64
	UsedMemory          uint64
	DiskTotal           uint64
	DiskUsed            uint64
	NumGC               uint32
	LastPauseNs         uint64
	GCCPUFraction       float64
	PoolActiveConns     int32
	PoolIdleConns       int32
	PoolTotalConns      int32
	PoolMaxConns        int32
	PoolAcquireCount    int64
	PoolNewConnsCount   int64
	PoolAcquireDuration time.Duration
	Hostname            string
	CurrentTime         time.Time
	Environment         string
	Timezone            string
	TimezoneOffset      string
	Version             string
	BranchName          string
	GitCommit           string
	BuildTime           string
	BuildEnvironment    string
	BuildGoVersion      string
}

type SystemInfoViewModel struct {
	PageContext  types.PageContextProvider
	Metrics      *SystemInfoMetrics
	Capabilities []sdkhealth.Capability

	FormattedHeapAlloc     string
	FormattedHeapSys       string
	FormattedUptime        string
	FormattedCPUPercent    string
	FormattedMemoryPercent string
	FormattedDiskPercent   string
	FormattedTotalMemory   string
	FormattedUsedMemory    string
	FormattedDiskTotal     string
	FormattedDiskUsed      string
	PoolUtilPercent        float64
}

func NewSystemInfoViewModel(metrics *SystemInfoMetrics, capabilities []sdkhealth.Capability, r *http.Request) *SystemInfoViewModel {
	pageCtx := composables.UsePageCtx(r.Context())

	if metrics == nil {
		metrics = &SystemInfoMetrics{}
	}

	vm := &SystemInfoViewModel{
		PageContext:  pageCtx,
		Metrics:      metrics,
		Capabilities: capabilities,
	}

	vm.FormattedHeapAlloc = formatBytes(metrics.HeapAlloc)
	vm.FormattedHeapSys = formatBytes(metrics.HeapSys)
	vm.FormattedUptime = formatDuration(metrics.Uptime)
	vm.FormattedCPUPercent = formatPercent(metrics.CPUPercent)
	vm.FormattedMemoryPercent = formatPercent(metrics.MemoryPercent)
	vm.FormattedDiskPercent = formatPercent(metrics.DiskPercent)
	vm.FormattedTotalMemory = formatBytes(metrics.TotalMemory)
	vm.FormattedUsedMemory = formatBytes(metrics.UsedMemory)
	vm.FormattedDiskTotal = formatBytes(metrics.DiskTotal)
	vm.FormattedDiskUsed = formatBytes(metrics.DiskUsed)

	if metrics.PoolMaxConns > 0 {
		vm.PoolUtilPercent = float64(metrics.PoolTotalConns) / float64(metrics.PoolMaxConns) * 100
	}

	return vm
}

// ThresholdColor returns a Tailwind background color class suffix
// based on resource utilization thresholds.
func ThresholdColor(percent float64) string {
	if percent < 60 {
		return "green-500"
	}
	if percent < 80 {
		return "yellow"
	}
	return "pink"
}

// PercentWidth clamps a float percentage to an integer in [0, 100]
// for use in inline style width attributes.
func PercentWidth(percent float64) int {
	w := int(percent)
	if w < 0 {
		return 0
	}
	if w > 100 {
		return 100
	}
	return w
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return "< 1 KB"
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KB", "MB", "GB", "TB", "PB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return formatFloat(float64(bytes)/float64(div)) + " " + units[exp]
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "< 1 min"
	}

	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return formatInt(days) + "d " + formatInt(hours) + "h " + formatInt(minutes) + "m"
	}
	if hours > 0 {
		return formatInt(hours) + "h " + formatInt(minutes) + "m"
	}
	return formatInt(minutes) + "m"
}

func formatPercent(percent float64) string {
	return formatFloat(percent) + "%"
}

func formatFloat(f float64) string {
	if f < 10 {
		return formatFloatPrecision(f, 1)
	}
	return formatFloatPrecision(f, 0)
}

func formatFloatPrecision(f float64, precision int) string {
	if precision == 0 {
		return formatInt(int(f))
	}
	intPart := int(f)
	fracPart := int((f - float64(intPart)) * 10)
	return formatInt(intPart) + "." + formatInt(fracPart)
}

func formatInt(i int) string {
	if i == 0 {
		return "0"
	}

	negative := i < 0
	if negative {
		i = -i
	}

	var result []byte
	for i > 0 {
		result = append([]byte{byte('0' + i%10)}, result...)
		i /= 10
	}

	if negative {
		result = append([]byte{'-'}, result...)
	}

	return string(result)
}
