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

// TemperatureInfo holds system temperature information
type TemperatureInfo struct {
	CPU float64 `json:"cpu"` // CPU temperature in Celsius
	GPU float64 `json:"gpu"` // GPU temperature in Celsius
}

// NetworkStats holds network statistics
type NetworkStats struct {
	BytesRecv   uint64  `json:"bytes_recv"`   // Total bytes received
	BytesSent   uint64  `json:"bytes_sent"`   // Total bytes sent
	DownloadMbps float64 `json:"download_mbps"` // Download speed in Mbps
	UploadMbps   float64 `json:"upload_mbps"`   // Upload speed in Mbps
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

// GetCPUTemperature returns the CPU temperature in Celsius using powermetrics
func GetCPUTemperature() (float64, error) {
	cmd := exec.Command("sudo", "powermetrics", "-n", "1", "-i", "1000", "--samplers", "smc")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative method using osx-cpu-temp if powermetrics fails
		return getCPUTempAlternative()
	}

	// Parse CPU die temperature from powermetrics output
	re := regexp.MustCompile(`CPU die temperature: ([\d.]+) C`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		temp, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return temp, nil
		}
	}

	return 0, fmt.Errorf("could not parse CPU temperature from powermetrics")
}

// getCPUTempAlternative tries to get CPU temperature using sysctl (less accurate but no sudo required)
func getCPUTempAlternative() (float64, error) {
	cmd := exec.Command("sysctl", "machdep.xcpm.cpu_thermal_level")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get CPU thermal level: %w", err)
	}

	// Parse thermal level (0-100, where higher means hotter)
	re := regexp.MustCompile(`machdep\.xcpm\.cpu_thermal_level: (\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		level, err := strconv.ParseInt(matches[1], 10, 64)
		if err == nil {
			// Rough estimate: convert thermal level to temperature (very approximate)
			// Thermal level 0 = ~40°C, level 100 = ~100°C
			estimatedTemp := 40.0 + (float64(level) * 0.6)
			return estimatedTemp, nil
		}
	}

	return 0, fmt.Errorf("could not parse CPU thermal level")
}

// GetGPUTemperature returns the GPU temperature in Celsius
func GetGPUTemperature() (float64, error) {
	cmd := exec.Command("sudo", "powermetrics", "-n", "1", "-i", "1000", "--samplers", "smc")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to run powermetrics: %w", err)
	}

	// Parse GPU die temperature from powermetrics output
	re := regexp.MustCompile(`GPU die temperature: ([\d.]+) C`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) >= 2 {
		temp, err := strconv.ParseFloat(matches[1], 64)
		if err == nil {
			return temp, nil
		}
	}

	// If no GPU temperature found, return 0 (some Macs don't have dedicated GPU)
	return 0, nil
}

// GetTemperatures returns CPU and GPU temperatures
func GetTemperatures() (*TemperatureInfo, error) {
	cpuTemp, cpuErr := GetCPUTemperature()
	gpuTemp, _ := GetGPUTemperature() // GPU error is not critical

	if cpuErr != nil {
		return nil, fmt.Errorf("failed to get CPU temperature: %w", cpuErr)
	}

	return &TemperatureInfo{
		CPU: cpuTemp,
		GPU: gpuTemp,
	}, nil
}

// GetNetworkStats returns current network statistics with speed calculation
func GetNetworkStats(lastStats *NetworkStats, interval time.Duration) (*NetworkStats, error) {
	// Get network interface statistics using netstat
	cmd := exec.Command("netstat", "-ibn")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run netstat: %w", err)
	}

	var totalBytesRecv uint64
	var totalBytesSent uint64

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 10 {
			continue
		}

		// Skip loopback and non-active interfaces
		ifName := fields[0]
		if strings.HasPrefix(ifName, "lo") || ifName == "Name" {
			continue
		}

		// Parse bytes received (column 7) and sent (column 10)
		bytesRecv, err1 := strconv.ParseUint(fields[6], 10, 64)
		bytesSent, err2 := strconv.ParseUint(fields[9], 10, 64)

		if err1 == nil && err2 == nil {
			totalBytesRecv += bytesRecv
			totalBytesSent += bytesSent
		}
	}

	// Calculate speeds if we have previous stats
	var downloadMbps, uploadMbps float64
	if lastStats != nil && interval.Seconds() > 0 {
		// Calculate bytes per second
		bytesRecvDiff := float64(totalBytesRecv - lastStats.BytesRecv)
		bytesSentDiff := float64(totalBytesSent - lastStats.BytesSent)

		// Convert to Mbps (megabits per second)
		downloadMbps = (bytesRecvDiff * 8) / (interval.Seconds() * 1000000)
		uploadMbps = (bytesSentDiff * 8) / (interval.Seconds() * 1000000)

		// Ensure non-negative values
		if downloadMbps < 0 {
			downloadMbps = 0
		}
		if uploadMbps < 0 {
			uploadMbps = 0
		}
	}

	return &NetworkStats{
		BytesRecv:   totalBytesRecv,
		BytesSent:   totalBytesSent,
		DownloadMbps: downloadMbps,
		UploadMbps:   uploadMbps,
	}, nil
}
