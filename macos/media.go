package macos

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
)

// MediaControlError represents an error when Media Control is not available
type MediaControlError struct {
	Message string
}

func (e *MediaControlError) Error() string {
	return e.Message
}

// MediaInfo represents the current media playing information
type MediaInfo struct {
	Title       string `json:"title"`
	Artist      string `json:"artist"`
	Album       string `json:"album"`
	AppName     string `json:"app_name"`
	AppBundleID string `json:"app_bundle_id"`
	State       string `json:"state"`    // "playing", "paused", "stopped"
	Duration    int    `json:"duration"` // in seconds
	Position    int    `json:"position"` // in seconds
}

// IsMediaControlAvailable checks if Media Control is installed and accessible
func IsMediaControlAvailable() bool {
	_, err := exec.LookPath("media-control")
	return err == nil
}

// GetMediaInfo retrieves current media information using Media Control
func GetMediaInfo() (*MediaInfo, error) {
	// Check if Media Control is available
	if !IsMediaControlAvailable() {
		return nil, &MediaControlError{Message: "Media Control is not installed or not accessible"}
	}

	// Get media information in JSON format
	out, err := exec.Command("media-control", "get").Output()
	if err != nil {
		return nil, fmt.Errorf("error getting media info: %v", err)
	}

	// Parse JSON output
	var mediaData map[string]interface{}
	if err := json.Unmarshal(out, &mediaData); err != nil {
		return nil, fmt.Errorf("error parsing media-control JSON output: %v", err)
	}

	// Check if media is playing
	playing, ok := mediaData["playing"].(bool)
	if !ok || !playing {
		return nil, nil // No media playing
	}

	// Extract media information
	mediaInfo := &MediaInfo{}

	// Get title
	if title, ok := mediaData["title"].(string); ok && title != "" {
		mediaInfo.Title = title
	}

	// Get artist
	if artist, ok := mediaData["artist"].(string); ok && artist != "" {
		mediaInfo.Artist = artist
	}

	// Get album
	if album, ok := mediaData["album"].(string); ok && album != "" {
		mediaInfo.Album = album
	}

	// Get app name
	if appName, ok := mediaData["appName"].(string); ok && appName != "" {
		mediaInfo.AppName = appName
	}

	// Get duration (in seconds)
	duration := 0
	if d, ok := mediaData["duration"].(float64); ok {
		duration = int(d)
	} else if d, ok := mediaData["durationMicros"].(float64); ok {
		duration = int(d / 1000000)
	} else if d, ok := mediaData["totalTime"].(float64); ok {
		duration = int(d)
	} else if d, ok := mediaData["totalDuration"].(float64); ok {
		duration = int(d)
	}
	mediaInfo.Duration = duration

	// Get position (in seconds)
	position := 0
	if p, ok := mediaData["elapsedTime"].(float64); ok {
		position = int(p)
	} else if p, ok := mediaData["position"].(float64); ok {
		position = int(p)
	} else if p, ok := mediaData["positionMicros"].(float64); ok {
		position = int(p / 1000000)
	}
	mediaInfo.Position = position

	// Set state based on playing status
	mediaInfo.State = "playing"

	return mediaInfo, nil
}

// TogglePlayPause toggles media play/pause
func TogglePlayPause() error {
	if !IsMediaControlAvailable() {
		return &MediaControlError{Message: "Media Control is not installed or not accessible"}
	}

	cmd := exec.Command("media-control", "toggle-play-pause")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error toggling play/pause: %w", err)
	}
	return nil
}

// LogMediaControlInstallInstructions logs installation instructions for Media Control
func LogMediaControlInstallInstructions() {
	log.Println("To install Media Control:")
	log.Println("  1. Install via npm: npm install -g media-control")
	log.Println("  2. Or install via Homebrew: brew install media-control")
	log.Println("Media player information will be disabled until Media Control is available")
}
