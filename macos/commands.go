package macos

import (
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// GetHostname returns the sanitized hostname
func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatal(err)
	}

	// "name.local" => "name"
	firstPart := strings.Split(hostname, ".")[0]

	// remove all symbols, but [a-zA-Z0-9_-]
	reg, err := regexp.Compile("[^a-zA-Z0-9_-]+")
	if err != nil {
		log.Fatal(err)
	}
	firstPart = reg.ReplaceAllString(firstPart, "")

	return firstPart
}

// GetSerialnumber returns the system serial number
func GetSerialnumber() string {
	cmd := "/usr/sbin/ioreg -l | /usr/bin/grep IOPlatformSerialNumber"
	output, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		log.Fatal(err)
	}
	outputStr := string(output)
	last := output[strings.LastIndex(outputStr, " ")+1:]
	lastStr := string(last)

	// remove all symbols, but [a-zA-Z0-9_-]
	reg, err := regexp.Compile("[^a-zA-Z0-9_-]+")
	if err != nil {
		log.Fatal(err)
	}
	lastStr = reg.ReplaceAllString(lastStr, "")

	return lastStr
}

// GetModel returns the system model/chip information
func GetModel() string {
	cmd := "/usr/sbin/system_profiler SPHardwareDataType |/usr/bin/grep Chip | /usr/bin/sed 's/\\(^.*: \\)\\(.*\\)/\\2/'"
	output, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		log.Fatal(err)
	}
	outputStr := string(output)
	outputStr = strings.TrimSuffix(outputStr, "\n")
	return outputStr
}

// GetWorkingDirectory returns the current working directory
func GetWorkingDirectory() string {
	wd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return wd
}

// GetCommandOutput executes a command and returns its output
func GetCommandOutput(name string, arg ...string) string {
	cmd := exec.Command(name, arg...)
	stdout, err := cmd.Output()
	if err != nil {
		log.Println("error: " + err.Error())
		log.Println("output: " + string(stdout))
		log.Fatal(err)
	}
	stdoutStr := string(stdout)
	stdoutStr = strings.TrimSuffix(stdoutStr, "\n")
	return stdoutStr
}

// GetCaffeinateStatus checks if caffeinate is running
func GetCaffeinateStatus() bool {
	cmd := "/bin/ps ax | /usr/bin/grep caffeinate | /usr/bin/grep -v grep"
	output, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		// Not running
	}
	stdoutStr := string(output)
	stdoutStr = strings.TrimSuffix(stdoutStr, "\n")
	return stdoutStr != ""
}

// RunCommand executes a command
func RunCommand(name string, arg ...string) {
	cmd := exec.Command(name, arg...)
	_, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
}

// Sleep puts the system to sleep
func Sleep() {
	RunCommand("pmset", "sleepnow")
}

// DisplaySleep puts displays to sleep
func DisplaySleep() {
	RunCommand("pmset", "displaysleepnow")
}

// Shutdown shuts down the system
func Shutdown() {
	if os.Getuid() == 0 {
		RunCommand("shutdown", "-h", "now")
	} else {
		RunCommand("/usr/bin/osascript", "-e", "tell app \"System Events\" to shut down")
	}
}

// DisplayWake wakes up the display
func DisplayWake() {
	RunCommand("/usr/bin/caffeinate", "-u", "-t", "1")
}

// KeepAwake prevents system sleep
func KeepAwake() {
	cmd := "/usr/bin/caffeinate -d &"
	err := exec.Command("/bin/sh", "-c", cmd).Start()
	if err != nil {
		log.Fatal(err)
	}
}

// AllowSleep allows the system to sleep again
func AllowSleep() {
	cmd := "/bin/ps ax | /usr/bin/grep caffeinate | /usr/bin/grep -v grep | /usr/bin/awk '{print \"kill \"$1}'|sh"
	_, err := exec.Command("/bin/sh", "-c", cmd).Output()
	if err != nil {
		log.Fatal(err)
	}
}

// RunShortcut runs a macOS shortcut
func RunShortcut(shortcut string) {
	RunCommand("shortcuts", "run", shortcut)
}

// Screensaver activates the screensaver
func Screensaver() {
	RunCommand("open", "-a", "ScreenSaverEngine")
}

// PlayPause toggles media play/pause
func PlayPause() {
	RunCommand("media-control", "toggle-play-pause")
}
