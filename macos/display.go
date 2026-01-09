package macos

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

// BetterDisplayCLIError represents an error when BetterDisplay CLI is not available
type BetterDisplayCLIError struct {
	Message string
}

func (e *BetterDisplayCLIError) Error() string {
	return e.Message
}

// Display represents the display information
type Display struct {
	UUID               string `json:"UUID"`
	AlphanumericSerial string `json:"alphanumericSerial"`
	DeviceType         string `json:"deviceType"`
	DisplayID          string `json:"displayID"`
	Model              string `json:"model"`
	Name               string `json:"name"`
	OriginalName       string `json:"originalName"`
	ProductName        string `json:"productName"`
	RegistryLocation   string `json:"registryLocation"`
	Serial             string `json:"serial"`
	TagID              string `json:"tagID"`
	Vendor             string `json:"vendor"`
	WeekOfManufacture  string `json:"weekOfManufacture"`
	YearOfManufacture  string `json:"yearOfManufacture"`
}

// IsBetterDisplayCLIAvailable checks if BetterDisplay CLI is installed and accessible
func IsBetterDisplayCLIAvailable() bool {
	_, err := exec.LookPath("betterdisplaycli")
	return err == nil
}

// GetDisplays retrieves all available displays using BetterDisplay CLI
func GetDisplays() []Display {
	// Check if BetterDisplay CLI is available
	if !IsBetterDisplayCLIAvailable() {
		log.Println("BetterDisplay CLI is not installed or not accessible")
		log.Println("To install BetterDisplay CLI:")
		log.Println("  1. Install BetterDisplay from https://github.com/waydabber/BetterDisplay")
		log.Println("  2. Enable CLI access in BetterDisplay preferences")
		log.Println("  3. Restart the application")
		log.Println("Display brightness controls will be disabled until BetterDisplay CLI is available")
		return nil
	}

	log.Println("Executing: betterdisplaycli get -identifiers")
	out, err := exec.Command("betterdisplaycli", "get", "-identifiers").Output()
	if err != nil {
		log.Printf("Error getting displays: %v", err)
		log.Println("BetterDisplay CLI is installed but failed to execute")
		log.Println("Make sure BetterDisplay is running and CLI access is enabled")
		return nil
	}

	log.Printf("BetterDisplay CLI output: %s", string(out))

	// BetterDisplay CLI returns comma-separated JSON objects, not an array
	// We need to wrap it in brackets to make it a valid JSON array
	jsonStr := "[" + string(out) + "]"

	var displays []Display
	err = json.Unmarshal([]byte(jsonStr), &displays)
	if err != nil {
		log.Printf("Error parsing display JSON: %v", err)
		log.Println("BetterDisplay CLI returned invalid JSON format")
		return nil
	}

	return displays
}

// IsDisplayAvailable checks if a display is currently available
func IsDisplayAvailable(displayID string) bool {
	// Get current display list to check if display is available
	displays := GetDisplays()
	if displays == nil {
		return false
	}

	for _, display := range displays {
		if display.DisplayID == displayID {
			return true
		}
	}
	return false
}

// GetDisplayBrightness gets the current brightness for a specific display (0-100)
func GetDisplayBrightness(displayID string) (int, error) {
	// First check if display is available to avoid unnecessary errors
	if !IsDisplayAvailable(displayID) {
		return 0, fmt.Errorf("display %s is not currently available", displayID)
	}

	out, err := exec.Command("betterdisplaycli", "get", "-displayID="+displayID, "-brightness", "-value").Output()
	if err != nil {
		return 0, fmt.Errorf("error getting brightness for display %s: %v", displayID, err)
	}

	// Parse the brightness value (0.0-1.0) and convert to percentage
	brightnessStr := strings.TrimSpace(string(out))
	brightness, err := strconv.ParseFloat(brightnessStr, 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing brightness value: %v", err)
	}

	return int(brightness * 100), nil
}

// SetDisplayBrightness sets the brightness for a specific display (0-100)
func SetDisplayBrightness(displayID string, brightness int) error {
	cmd := exec.Command("betterdisplaycli", "set", "-displayID="+displayID, "-brightness="+strconv.Itoa(brightness)+"%")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error setting brightness for display %s: %v", displayID, err)
	}
	return nil
}
