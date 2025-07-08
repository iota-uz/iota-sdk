package services

import (
	"github.com/shirou/gopsutil/v4/process"
)

// ResourceUsage represents CPU and memory usage for a process
type ResourceUsage struct {
	CPUPercent float64
	MemoryMB   float64
}

// GetProcessResourceUsage returns CPU and memory usage for a given PID
func GetProcessResourceUsage(pid int) (*ResourceUsage, error) {
	if pid <= 0 {
		return &ResourceUsage{}, nil
	}

	// Create process instance
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		// Process might not exist
		return &ResourceUsage{}, err
	}

	// Get CPU percent (this is calculated over a short interval)
	cpuPercent, err := proc.CPUPercent()
	if err != nil {
		cpuPercent = 0
	}

	// Get memory info
	memInfo, err := proc.MemoryInfo()
	if err != nil {
		return &ResourceUsage{
			CPUPercent: cpuPercent,
			MemoryMB:   0,
		}, err
	}

	// RSS is in bytes, convert to MB
	memoryMB := float64(memInfo.RSS) / (1024 * 1024)

	return &ResourceUsage{
		CPUPercent: cpuPercent,
		MemoryMB:   memoryMB,
	}, nil
}

// GetProcessTree returns all child PIDs for a given parent PID
func GetProcessTree(parentPID int) ([]int, error) {
	if parentPID <= 0 {
		return nil, nil
	}

	// Create process instance
	proc, err := process.NewProcess(int32(parentPID))
	if err != nil {
		return nil, err
	}

	// Get all child processes
	children, err := proc.Children()
	if err != nil {
		return nil, err
	}

	pids := make([]int, 0, len(children))
	for _, child := range children {
		pids = append(pids, int(child.Pid))
	}

	return pids, nil
}

// GetTotalResourceUsage returns combined resource usage for a process and all its children
func GetTotalResourceUsage(pid int) (*ResourceUsage, error) {
	if pid <= 0 {
		return &ResourceUsage{}, nil
	}

	// Get resource usage for the main process
	mainUsage, err := GetProcessResourceUsage(pid)
	if err != nil {
		return &ResourceUsage{}, err
	}

	totalCPU := mainUsage.CPUPercent
	totalMemory := mainUsage.MemoryMB

	// Get child processes
	childPIDs, err := GetProcessTree(pid)
	if err == nil && len(childPIDs) > 0 {
		// Add resource usage from child processes
		for _, childPID := range childPIDs {
			childUsage, err := GetProcessResourceUsage(childPID)
			if err == nil {
				totalCPU += childUsage.CPUPercent
				totalMemory += childUsage.MemoryMB
			}
		}
	}

	return &ResourceUsage{
		CPUPercent: totalCPU,
		MemoryMB:   totalMemory,
	}, nil
}
