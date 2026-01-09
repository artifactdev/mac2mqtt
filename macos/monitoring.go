package macos

import (
	"context"
	"fmt"
	"log"
	"net"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	mediadevices "github.com/antonfisher/go-media-devices-state"
	sigar "github.com/cloudfoundry/gosigar"
	mem "github.com/shirou/gopsutil/v3/mem"
)

// DiskUsage holds disk usage statistics
type DiskUsage struct {
	Total       uint64  `json:"total"`        // Total bytes
	Used        uint64  `json:"used"`         // Used bytes
	Free        uint64  `json:"free"`         // Free bytes
	UsedPercent float64 `json:"used_percent"` // Used percentage
	FreePercent float64 `json:"free_percent"` // Free percentage
}

// CPUUsage holds CPU usage statistics
type CPUUsage struct {
	UsedPercent float64 `json:"used_percent"` // CPU used percentage
	FreePercent float64 `json:"free_percent"` // CPU idle/free percentage
}

// MemoryUsage holds memory usage statistics
type MemoryUsage struct {
	Total       uint64  `json:"total"`        // Total bytes
	Used        uint64  `json:"used"`         // Used bytes
	Free        uint64  `json:"free"`         // Free bytes
	UsedPercent float64 `json:"used_percent"` // Used percentage
	FreePercent float64 `json:"free_percent"` // Free percentage
}

// UptimeInfo holds system uptime information
type UptimeInfo struct {
	Seconds uint64 `json:"seconds"` // Uptime in seconds
	Human   string `json:"human"`   // Human-readable format
}

// GetDiskUsage returns disk usage statistics for the root filesystem
func GetDiskUsage() (*DiskUsage, error) {
	fs := sigar.FileSystemList{}
	if err := fs.Get(); err != nil {
		return nil, fmt.Errorf("failed to get filesystem list: %w", err)
	}

	// Find the root filesystem
	for _, filesystem := range fs.List {
		if filesystem.DirName == "/" {
			usage := sigar.FileSystemUsage{}
			if err := usage.Get(filesystem.DirName); err != nil {
				return nil, fmt.Errorf("failed to get disk usage: %w", err)
			}

			// Convert from KB to bytes (gosigar returns values in KB)
			totalBytes := usage.Total * 1024
			usedBytes := usage.Used * 1024
			freeBytes := usage.Free * 1024

			// Calculate percentages
			usedPercent := float64(0)
			freePercent := float64(0)
			if totalBytes > 0 {
				usedPercent = float64(usedBytes) / float64(totalBytes) * 100
				freePercent = float64(freeBytes) / float64(totalBytes) * 100
			}

			return &DiskUsage{
				Total:       totalBytes,
				Used:        usedBytes,
				Free:        freeBytes,
				UsedPercent: usedPercent,
				FreePercent: freePercent,
			}, nil
		}
	}

	return nil, fmt.Errorf("root filesystem not found")
}

// GetCPUUsage calculates CPU usage based on delta since last measurement
func GetCPUUsage(lastCPU *sigar.Cpu) (*CPUUsage, *sigar.Cpu, error) {
	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		return nil, nil, fmt.Errorf("failed to get CPU stats: %w", err)
	}

	// If this is the first call and lastCPU is nil, initialize it
	if lastCPU == nil {
		return &CPUUsage{
			UsedPercent: 0,
			FreePercent: 100,
		}, &cpu, nil
	}

	// Calculate the delta since last measurement
	userDelta := cpu.User - lastCPU.User
	sysDelta := cpu.Sys - lastCPU.Sys
	idleDelta := cpu.Idle - lastCPU.Idle
	waitDelta := cpu.Wait - lastCPU.Wait
	niceDelta := cpu.Nice - lastCPU.Nice
	irqDelta := cpu.Irq - lastCPU.Irq
	softIrqDelta := cpu.SoftIrq - lastCPU.SoftIrq
	stolenDelta := cpu.Stolen - lastCPU.Stolen

	// Calculate total time delta
	totalDelta := userDelta + sysDelta + idleDelta + waitDelta + niceDelta + irqDelta + softIrqDelta + stolenDelta

	// If total is zero, return 0% usage
	if totalDelta == 0 {
		return &CPUUsage{
			UsedPercent: 0,
			FreePercent: 100,
		}, &cpu, nil
	}

	// Calculate idle and used percentages
	idlePercent := float64(idleDelta) / float64(totalDelta) * 100
	usedPercent := 100 - idlePercent

	return &CPUUsage{
		UsedPercent: usedPercent,
		FreePercent: idlePercent,
	}, &cpu, nil
}

// GetMemoryUsage returns memory usage statistics
func GetMemoryUsage() (*MemoryUsage, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("error getting virtual memory stats: %w", err)
	}

	total := vmStat.Total
	used := vmStat.Used
	free := vmStat.Free

	return &MemoryUsage{
		Total:       total,
		Used:        used,
		Free:        free,
		UsedPercent: vmStat.UsedPercent,
		FreePercent: 100 - vmStat.UsedPercent,
	}, nil
}

// GetSystemUptime returns system uptime information
func GetSystemUptime() (*UptimeInfo, error) {
	uptime := sigar.Uptime{}
	if err := uptime.Get(); err != nil {
		return nil, fmt.Errorf("failed to get uptime: %w", err)
	}

	// Convert to human-readable format
	totalSeconds := uint64(uptime.Length)
	days := totalSeconds / 86400
	hours := (totalSeconds % 86400) / 3600
	minutes := (totalSeconds % 3600) / 60

	var uptimeHuman string
	if days > 0 {
		uptimeHuman = fmt.Sprintf("%d days, %d:%02d", days, hours, minutes)
	} else {
		uptimeHuman = fmt.Sprintf("%d:%02d", hours, minutes)
	}

	return &UptimeInfo{
		Seconds: totalSeconds,
		Human:   uptimeHuman,
	}, nil
}

// GetBatteryChargePercent returns the battery charge percentage as a string
func GetBatteryChargePercent() string {
	cmd := exec.Command("/usr/bin/pmset", "-g", "batt")
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Error getting battery charge: %v", err)
		return ""
	}

	// $ /usr/bin/pmset -g batt
	// Now drawing from 'Battery Power'
	//  -InternalBattery-0 (id=4653155)        100%; discharging; 20:00 remaining present: true

	r := regexp.MustCompile(`(\d+)%`)
	res := r.FindStringSubmatch(string(output))
	if len(res) == 0 {
		return ""
	}

	return res[1]
}

// GetSystemIdleTime gets the system idle time in seconds
func GetSystemIdleTime() (int, error) {
	cmd := exec.Command("ioreg", "-c", "IOHIDSystem")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("error running ioreg: %w", err)
	}

	// Parse the HIDIdleTime from the output
	re := regexp.MustCompile(`"HIDIdleTime" = (\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) < 2 {
		return 0, fmt.Errorf("HIDIdleTime not found in ioreg output")
	}

	idleTimeNanos, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing idle time: %w", err)
	}

	// Convert nanoseconds to seconds
	idleTimeSeconds := int(idleTimeNanos / 1000000000)
	return idleTimeSeconds, nil
}

// GetMediaDevicesState returns the state of microphone and camera
func GetMediaDevicesState() (isMicOn bool, isCameraOn bool, err error) {
	isMicOn, err = mediadevices.IsMicrophoneOn()
	if err != nil {
		return false, false, fmt.Errorf("failed to get microphone state: %w", err)
	}

	isCameraOn, err = mediadevices.IsCameraOn()
	if err != nil {
		return isMicOn, false, fmt.Errorf("failed to get camera state: %w", err)
	}

	return isMicOn, isCameraOn, nil
}

// GetPublicIP returns the public IP address of the system
func GetPublicIP() (string, error) {
	// Use DNS to query Google's whoami service
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: 5 * time.Second,
			}
			return d.DialContext(ctx, "udp", "ns1.google.com:53")
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Query TXT record from Google's whoami service
	txtRecords, err := resolver.LookupTXT(ctx, "o-o.myaddr.l.google.com")
	if err != nil {
		return "", fmt.Errorf("failed to lookup public IP via DNS: %w", err)
	}

	if len(txtRecords) == 0 {
		return "", fmt.Errorf("no TXT records found for public IP")
	}

	// The first TXT record contains the IP address
	publicIP := strings.TrimSpace(txtRecords[0])
	return publicIP, nil
}
