package macos

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

// GetMuteStatus returns the current mute status of the system
func GetMuteStatus() bool {
	log.Println("Getting mute status")
	output := getCommandOutput("/usr/bin/osascript", "-e", "output muted of (get volume settings)")
	b, err := strconv.ParseBool(output)
	//revive:disable-next-line
	if err != nil {
		// Continue to fallback method
	}
	if output == "missing value" {
		currentsource := getCommandOutput("/opt/homebrew/bin/switchaudiosource", "-c")
		var resp *http.Response
		var err error

		// URL encode the current source name to handle spaces and special characters
		encodedSource := strings.ReplaceAll(currentsource, " ", "%20")
		url := fmt.Sprintf("http://localhost:55777/get?name=%s&mute", encodedSource)
		resp, err = http.Get(url)
		if err != nil {
			log.Printf("Error getting mute status for %s: %v", currentsource, err)
			return false
		}
		if resp != nil {
			defer resp.Body.Close()
			output, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error getting mute status body for %s: %v", currentsource, err)
				return false
			}
			output = []byte(strings.TrimSuffix(string(output), "\n"))
			mute := string(output)
			log.Println("Mute Output: " + mute)
			b = mute == "on"
		}
	}
	return b
}

// GetVolume returns the current volume level (0-100)
func GetVolume() int {
	log.Println("Getting volume status")
	output := getCommandOutput("/usr/bin/osascript", "-e", "output volume of (get volume settings)")
	output = strings.TrimSuffix(output, "\n")
	i, err := strconv.Atoi(output)
	if err != nil {
		currentsource := getCommandOutput("/opt/homebrew/bin/switchaudiosource", "-c")
		var resp *http.Response
		var err error
		// URL encode the current source name to handle spaces and special characters
		encodedSource := strings.ReplaceAll(currentsource, " ", "%20")
		url := fmt.Sprintf("http://localhost:55777/get?name=%s&volume", encodedSource)
		resp, err = http.Get(url)
		if err != nil {
			log.Printf("Error getting volume status for %s: %v", currentsource, err)
			return 0
		}
		if resp != nil {
			defer resp.Body.Close()
			output, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Printf("Error getting volume status body for %s: %v", currentsource, err)
				return 0
			}
			output = []byte(strings.TrimSuffix(string(output), "\n"))
			outputStr := string(output)
			log.Println("Vol Output: " + outputStr)
			f, err := strconv.ParseFloat(outputStr, 64)
			if err != nil {
				log.Printf("Error parsing volume value for %s: %v", currentsource, err)
				return 0
			}
			i = int(f * 100)
		}
	}
	return i
}

// SetVolume sets the system volume (0-100)
func SetVolume(i int) error {
	//Test first if we can control the volume if not use switchaudiosource
	test := getCommandOutput("/usr/bin/osascript", "-e", "output volume of (get volume settings)")
	if test == "missing value" {
		volumef := float64(i) / 100
		currentsource := getCommandOutput("/opt/homebrew/bin/switchaudiosource", "-c")
		// URL encode the current source name to handle spaces and special characters
		encodedSource := strings.ReplaceAll(currentsource, " ", "%20")
		url := fmt.Sprintf("http://localhost:55777/set?name=%s&volume=%f", encodedSource, volumef)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("error setting volume for %s: %w", currentsource, err)
		}
		if resp != nil {
			resp.Body.Close()
		}
	} else {
		cmd := exec.Command("/usr/bin/osascript", "-e", "set volume output volume "+strconv.Itoa(i))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error setting volume: %w", err)
		}
	}
	return nil
}

// SetMute sets the mute status (true = muted, false = unmuted)
func SetMute(b bool) error {
	//Test first if we can control the mute if not use switchaudiosource
	test := getCommandOutput("/usr/bin/osascript", "-e", "output volume of (get volume settings)")
	if test == "missing value" {
		state := "off"
		if b {
			state = "on"
		}
		currentsource := getCommandOutput("/opt/homebrew/bin/switchaudiosource", "-c")
		// URL encode the current source name to handle spaces and special characters
		encodedSource := strings.ReplaceAll(currentsource, " ", "%20")
		url := fmt.Sprintf("http://localhost:55777/set?name=%s&mute=%s", encodedSource, state)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("error setting mute for %s: %w", currentsource, err)
		}
		if resp != nil {
			resp.Body.Close()
		}
	} else {
		cmd := exec.Command("/usr/bin/osascript", "-e", "set volume output muted "+strconv.FormatBool(b))
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("error setting mute: %w", err)
		}
	}
	return nil
}

// getCommandOutput runs a command and returns its output as a string
func getCommandOutput(name string, arg ...string) string {
	cmd := exec.Command(name, arg...)
	stdout, err := cmd.Output()
	if err != nil {
		log.Println("error: " + err.Error())
		log.Println("output: " + string(stdout))
		return ""
	}
	stdoutStr := string(stdout)
	stdoutStr = strings.TrimSuffix(stdoutStr, "\n")
	return stdoutStr
}
