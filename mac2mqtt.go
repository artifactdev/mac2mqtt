// main - obigatory main comment for package to appease the linting gods
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"bessarabov/mac2mqtt/macos"

	"gopkg.in/yaml.v2"

	sigar "github.com/cloudfoundry/gosigar"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

// Constants for the application
const (
	DefaultDiscoveryPrefix = "homeassistant"
	DefaultTopicPrefix     = "mac2mqtt"
	UpdateInterval         = 60 * time.Second
	MaxVolume              = 100
	MinVolume              = 0
	MaxBrightness          = 100
	MinBrightness          = 0
	MaxRetryAttempts       = 1
)

// BetterDisplayCLIError represents an error when BetterDisplay CLI is not available
type BetterDisplayCLIError struct {
	message string
}

func (e *BetterDisplayCLIError) Error() string {
	return e.message
}

// MediaControlError represents an error when Media Control is not available
type MediaControlError struct {
	message string
}

func (e *MediaControlError) Error() string {
	return e.message
}

// Type aliases for convenience
type MediaInfo = macos.MediaInfo
type Display = macos.Display

// Application holds the main application state
type Application struct {
	config            *config
	displays          []macos.Display
	hostname          string
	topic             string
	client            mqtt.Client
	currentMediaState macos.MediaInfo // persistent media state for streaming
	userActivityState string           // "active" or "inactive"
	activityMutex     sync.RWMutex
	activityTimer     *time.Timer
	lastCPU           sigar.Cpu // for CPU percentage calculation
	cpuMutex          sync.RWMutex
	lmstudioServerRunning bool                 // LM Studio server status
	lmstudioLoadedModels  []macos.LMStudioModel // Currently loaded models
	lmstudioMutex        sync.RWMutex
}

type config struct {
	IP               string `yaml:"mqtt_ip"`
	Port             string `yaml:"mqtt_port"`
	User             string `yaml:"mqtt_user"`
	Password         string `yaml:"mqtt_password"`
	SSL              bool   `yaml:"mqtt_ssl"`
	Hostname         string `yaml:"hostname"`
	Topic            string `yaml:"mqtt_topic"`
	DiscoveryPrefix  string `yaml:"discovery_prefix"`
	IdleActivityTime int    `yaml:"idle_activity_time"` // in seconds
	LMStudioEnabled  bool   `yaml:"lmstudio_enabled"`   // Enable LM Studio integration
	LMStudioAPIURL   string `yaml:"lmstudio_api_url"`   // LM Studio API URL (default: http://localhost:1234)
}

func (c *config) getConfig() *config {

	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)

	log.Printf("Path: %v", exPath)
	configContent, err := os.ReadFile(exPath + "/mac2mqtt.yaml")
	if err != nil {
		log.Fatal("No config file provided")
	}

	err = yaml.Unmarshal(configContent, c)
	if err != nil {
		log.Fatal("No data in config file")
	}

	if c.IP == "" {
		log.Fatal("Must specify mqtt_ip in mac2mqtt.yaml")
	}

	if c.IdleActivityTime == 0 {
		log.Println("No idle_activity_time specified in config, using default 10 seconds")

	}

	if c.Port == "" {
		log.Fatal("Must specify mqtt_port in mac2mqtt.yaml")
	}

	if c.Hostname == "" {
		c.Hostname = macos.GetHostname()
	}
	if c.DiscoveryPrefix == "" {
		c.DiscoveryPrefix = "homeassistant"
	}
	if c.LMStudioAPIURL == "" {
		c.LMStudioAPIURL = "http://localhost:1234"
	}
	return c
}

// NewApplication creates and initializes a new Application instance
func NewApplication() (*Application, error) {
	app := &Application{}

	// Load configuration
	app.config = &config{}
	app.config.getConfig()

	// Set hostname and sanitize it (remove spaces and special characters for MQTT topics)
	if app.config.Hostname == "" {
		app.hostname = macos.GetHostname()
	} else {
		app.hostname = app.config.Hostname
	}

	// Sanitize hostname for use in MQTT topics (remove spaces and special characters)
	sanitizedHostname := strings.ReplaceAll(app.hostname, " ", "")
	sanitizedHostname = strings.ReplaceAll(sanitizedHostname, "/", "")
	sanitizedHostname = strings.ReplaceAll(sanitizedHostname, "+", "")
	sanitizedHostname = strings.ReplaceAll(sanitizedHostname, "#", "")

	// Set topic - append sanitized hostname to allow multiple instances
	if app.config.Topic == "" {
		app.topic = DefaultTopicPrefix + "/" + sanitizedHostname
	} else {
		// Append sanitized hostname to the configured topic
		app.topic = app.config.Topic + "/" + sanitizedHostname
	}

	// Validate configuration
	if err := app.validateConfig(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	// Initialize displays
	app.displays = macos.GetDisplays()

	// Initialize currentMediaState
	if macos.IsMediaControlAvailable() {
		mediaInfo, err := macos.GetMediaInfo()
		if err == nil && mediaInfo != nil {
			app.currentMediaState = *mediaInfo
		} else {
			app.currentMediaState = macos.MediaInfo{State: "idle"}
		}
	} else {
		app.currentMediaState = macos.MediaInfo{State: "idle"}
	}

	// Initialize user activity state
	app.userActivityState = "inactive"

	// Initialize CPU stats for percentage calculation
	if err := app.lastCPU.Get(); err != nil {
		log.Printf("Warning: Failed to initialize CPU stats: %v", err)
	}

	return app, nil
}

// validateConfig validates the application configuration
func (app *Application) validateConfig() error {
	if app.config.IP == "" {
		return fmt.Errorf("mqtt_ip is required")
	}
	if app.config.Port == "" {
		return fmt.Errorf("mqtt_port is required")
	}
	if app.config.DiscoveryPrefix == "" {
		app.config.DiscoveryPrefix = DefaultDiscoveryPrefix
	}
	return nil
}

// getTopicPrefix returns the topic prefix for this application
func (app *Application) getTopicPrefix() string {
	return app.topic
}









// updateMediaPlayer updates the MQTT topics with current media player information
func (app *Application) updateMediaPlayer(client mqtt.Client) {
	mediaInfo, err := macos.GetMediaInfo()
	if err != nil {
		// Check if it's a Media Control error
		if _, ok := err.(*MediaControlError); ok {
			log.Printf("Media Control is not available: %v", err)
			log.Println("To install Media Control:")
			log.Println("  1. Install via npm: npm install -g media-control")
			log.Println("  2. Or install via Homebrew: brew install media-control")
			log.Println("Media player information will be disabled until Media Control is available")
		} else {
			log.Printf("Error getting media info: %v", err)
		}
		return
	}

	// If no media is playing, publish empty state
	if mediaInfo == nil {
		log.Println("No media playing - publishing idle state")
		app.publishMediaState(client, "idle", "", "", "", "", 0, 0)
		return
	}

	// Determine the state
	state := "idle"
	switch mediaInfo.State {
	case "playing":
		state = "playing"
	case "paused":
		state = "paused"
	case "stopped":
		state = "idle"
	}

	log.Printf("Media playing: %s - %s (%s)", mediaInfo.Artist, mediaInfo.Title, state)
	app.publishMediaState(client, state, mediaInfo.Title, mediaInfo.Artist, mediaInfo.Album, mediaInfo.AppName, mediaInfo.Duration, mediaInfo.Position)
}

// updateNowPlaying updates the now playing sensor with current media information
func (app *Application) updateNowPlaying(client mqtt.Client) {
	mediaInfo, err := macos.GetMediaInfo()
	if err != nil {
		if _, ok := err.(*MediaControlError); ok {
			log.Printf("Media Control is not available: %v", err)
			return
			//revive:disable-next-line
		} else {
			log.Printf("Error getting media info: %v", err)
			return
		}
	}

	// If no media is playing, publish idle state
	if mediaInfo == nil {
		state := "idle"
		client.Publish(app.getTopicPrefix()+"/status/now_playing", 0, false, state)
		attr := map[string]interface{}{
			"state":    state,
			"title":    "",
			"artist":   "",
			"album":    "",
			"app_name": "",
			"duration": 0,
			"position": 0,
		}
		attrJSON, _ := json.Marshal(attr)
		client.Publish(app.getTopicPrefix()+"/status/now_playing_attr", 0, false, string(attrJSON))
		return
	}

	// Determine the state
	state := "idle"
	switch mediaInfo.State {
	case "playing":
		state = "playing"
	case "paused":
		state = "paused"
	case "stopped":
		state = "idle"
	}

	// Publish state and attributes
	client.Publish(app.getTopicPrefix()+"/status/now_playing", 0, false, state)
	attr := map[string]interface{}{
		"state":    state,
		"title":    mediaInfo.Title,
		"artist":   mediaInfo.Artist,
		"album":    mediaInfo.Album,
		"app_name": mediaInfo.AppName,
		"duration": mediaInfo.Duration,
		"position": mediaInfo.Position,
	}
	attrJSON, _ := json.Marshal(attr)
	client.Publish(app.getTopicPrefix()+"/status/now_playing_attr", 0, false, string(attrJSON))
	log.Printf("Updated now playing sensor: %s - %s (%s)", mediaInfo.Artist, mediaInfo.Title, state)
}

// startMediaStream starts the media-control stream for real-time updates
func (app *Application) startMediaStream(client mqtt.Client) {
	if !macos.IsMediaControlAvailable() {
		log.Println("Media Control not available - skipping media stream")
		return
	}

	log.Println("Starting media-control stream for real-time updates...")

	cmd := exec.Command("media-control", "stream")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("Error creating stdout pipe for media stream: %v", err)
		return
	}

	if err := cmd.Start(); err != nil {
		log.Printf("Error starting media-control stream: %v", err)
		return
	}

	// Read the stream in a goroutine with error recovery
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Media stream goroutine recovered from panic: %v", r)
			}
			cmd.Wait()
			stdout.Close()
		}()

		scanner := bufio.NewScanner(stdout)
		// Increase buffer size to handle long JSON lines from media-control stream
		buf := make([]byte, 0, 64*1024) // 64KB buffer
		scanner.Buffer(buf, 1024*1024)  // Allow up to 1MB tokens

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}

			// Parse the JSON line from the stream
			var mediaData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &mediaData); err != nil {
				log.Printf("Error parsing media stream JSON: %v", err)
				continue
			}

			// Process the media update only if MQTT client is connected
			if client.IsConnected() {
				app.processMediaStreamUpdate(client, mediaData)
			}
		}

		if err := scanner.Err(); err != nil {
			log.Printf("Error reading media stream: %v", err)
			log.Println("Media stream will restart on next application restart")
		}
	}()

	log.Println("Media stream started successfully")
}

// processMediaStreamUpdate processes a single media update from the stream
func (app *Application) processMediaStreamUpdate(client mqtt.Client, mediaData map[string]interface{}) {
	// The stream sends {"type":"data","diff":true,"payload":{...}}
	// Only update fields present in payload
	payload, ok := mediaData["payload"].(map[string]interface{})
	if !ok {
		log.Printf("Media stream: No payload in event, skipping")
		return
	}

	// Merge payload into currentMediaState
	for k, v := range payload {
		switch k {
		case "title":
			if s, ok := v.(string); ok {
				app.currentMediaState.Title = s
			}
		case "artist":
			if s, ok := v.(string); ok {
				app.currentMediaState.Artist = s
			}
		case "album":
			if s, ok := v.(string); ok {
				app.currentMediaState.Album = s
			}
		case "appName":
			if s, ok := v.(string); ok {
				app.currentMediaState.AppName = s
			}
		case "bundleIdentifier":
			if s, ok := v.(string); ok {
				app.currentMediaState.AppName = s
			}
		case "playing":
			if b, ok := v.(bool); ok {
				if b {
					app.currentMediaState.State = "playing"
				} else {
					app.currentMediaState.State = "paused"
				}
			}
		case "duration":
			if f, ok := v.(float64); ok {
				app.currentMediaState.Duration = int(f)
			}
		case "durationMicros":
			if f, ok := v.(float64); ok {
				app.currentMediaState.Duration = int(f / 1000000)
			}
		case "totalTime":
			if f, ok := v.(float64); ok {
				app.currentMediaState.Duration = int(f)
			}
		case "totalDuration":
			if f, ok := v.(float64); ok {
				app.currentMediaState.Duration = int(f)
			}
		case "elapsedTime":
			if f, ok := v.(float64); ok {
				app.currentMediaState.Position = int(f)
			}
		case "position":
			if f, ok := v.(float64); ok {
				app.currentMediaState.Position = int(f)
			}
		case "positionMicros":
			if f, ok := v.(float64); ok {
				app.currentMediaState.Position = int(f / 1000000)
			}
		}
	}

	// If playing is false and no other info, treat as idle
	if state, ok := payload["playing"]; ok {
		if b, ok := state.(bool); ok && !b {
			app.currentMediaState.State = "idle"
		}
	}

	// Publish state and attributes
	client.Publish(app.getTopicPrefix()+"/status/now_playing", 0, false, app.currentMediaState.State)
	attr := map[string]interface{}{
		"state":    app.currentMediaState.State,
		"title":    app.currentMediaState.Title,
		"artist":   app.currentMediaState.Artist,
		"album":    app.currentMediaState.Album,
		"app_name": app.currentMediaState.AppName,
		"duration": app.currentMediaState.Duration,
		"position": app.currentMediaState.Position,
	}
	attrJSON, _ := json.Marshal(attr)
	client.Publish(app.getTopicPrefix()+"/status/now_playing_attr", 0, false, string(attrJSON))
	log.Printf("Media stream update: %s - %s (%s)", app.currentMediaState.Artist, app.currentMediaState.Title, app.currentMediaState.State)
}

// getUserActivityState gets the current user activity state
func (app *Application) getUserActivityState() string {
	app.activityMutex.RLock()
	defer app.activityMutex.RUnlock()
	return app.userActivityState
}

// setUserActivityState sets the user activity state and publishes to MQTT
func (app *Application) setUserActivityState(client mqtt.Client, state string) {
	app.activityMutex.Lock()
	defer app.activityMutex.Unlock()

	if app.userActivityState != state {
		app.userActivityState = state
		if client != nil && client.IsConnected() {
			client.Publish(app.getTopicPrefix()+"/status/user_activity", 0, false, state)
			log.Printf("User activity state changed to: %s", state)
		}
	}
}

// resetActivityTimer resets the inactivity timer
func (app *Application) resetActivityTimer(client mqtt.Client) {
	app.activityMutex.Lock()
	defer app.activityMutex.Unlock()

	// Set to active immediately
	if app.userActivityState != "active" {
		app.userActivityState = "active"
		if client != nil && client.IsConnected() {
			client.Publish(app.getTopicPrefix()+"/status/user_activity", 0, false, "active")
			log.Printf("User activity detected - state: active")
		}
	}

	// Reset or create the timer
	if app.activityTimer != nil {
		app.activityTimer.Stop()
	}

	app.activityTimer = time.AfterFunc(time.Duration(app.config.IdleActivityTime)*time.Second, func() {
		app.setUserActivityState(client, "inactive")
	})
}

// startUserActivityMonitoring starts monitoring user activity using system idle time
func (app *Application) startUserActivityMonitoring(client mqtt.Client) {
	log.Println("Starting user activity monitoring...")

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Activity monitor goroutine recovered from panic: %v", r)
			}
		}()

		var lastIdleTime int = -1

		for {
			// Check if client is still connected
			if client == nil || !client.IsConnected() {
				time.Sleep(5 * time.Second)
				continue
			}

			idleTime, err := macos.GetSystemIdleTime()
			if err != nil {
				log.Printf("Error getting system idle time: %v", err)
				time.Sleep(2 * time.Second)
				continue
			}

			// If idle time decreased or is very small, user is active
			if idleTime < lastIdleTime || idleTime < 2 {
				app.resetActivityTimer(client)
			}

			lastIdleTime = idleTime
			client.Publish(app.getTopicPrefix()+"/status/idle_time_seconds", 0, false, fmt.Sprintf("%d", idleTime))
			// Check every 500ms for responsive detection
			time.Sleep(500 * time.Millisecond)
		}
	}()

	log.Println("User activity monitoring started successfully")
}

// publishMediaState publishes the current media state to MQTT
func (app *Application) publishMediaState(client mqtt.Client, state, title, artist, album, appName string, duration, position int) {
	// Publish individual attributes
	client.Publish(app.getTopicPrefix()+"/status/media_state", 0, false, state)
	client.Publish(app.getTopicPrefix()+"/status/media_title", 0, false, title)
	client.Publish(app.getTopicPrefix()+"/status/media_artist", 0, false, artist)
	client.Publish(app.getTopicPrefix()+"/status/media_album", 0, false, album)
	client.Publish(app.getTopicPrefix()+"/status/media_app", 0, false, appName)
	client.Publish(app.getTopicPrefix()+"/status/media_duration", 0, false, strconv.Itoa(duration))
	client.Publish(app.getTopicPrefix()+"/status/media_position", 0, false, strconv.Itoa(position))

	// Publish combined JSON state for media_player entity
	mediaState := map[string]interface{}{
		"state":        state,
		"title":        title,
		"artist":       artist,
		"album":        album,
		"app_name":     appName,
		"duration":     duration,
		"position":     position,
		"media_title":  title,
		"media_artist": artist,
		"media_album":  album,
	}

	stateJSON, _ := json.Marshal(mediaState)
	mediaPlayerTopic := app.getTopicPrefix() + "/status/media_player"
	client.Publish(mediaPlayerTopic, 0, false, string(stateJSON))
	log.Printf("Published media state to %s: %s", mediaPlayerTopic, string(stateJSON))
}

// updateDisplayBrightness updates the MQTT topics with current display brightness values
func (app *Application) updateDisplayBrightness(client mqtt.Client) {
	// Skip if no displays are available
	if len(app.displays) == 0 {
		return
	}

	// Refresh display list to handle dynamic display changes (laptop open/close)
	currentDisplays := macos.GetDisplays()
	if currentDisplays != nil {
		app.displays = currentDisplays
	}

	for _, display := range app.displays {
		brightness, err := macos.GetDisplayBrightness(display.DisplayID)
		if err != nil {
			// Only log error once per minute to avoid spam for unavailable displays (e.g., closed laptop)
			if display.Name == "Built-in Display" || strings.Contains(display.Name, "Built-in") {
				// Silently skip built-in display when unavailable (laptop closed)
				continue
			}
			log.Printf("Error getting brightness for display %s: %v", display.Name, err)
			// Check if it's a BetterDisplay CLI error
			if !macos.IsBetterDisplayCLIAvailable() {
				log.Printf("BetterDisplay CLI is not available for display %s", display.Name)
			}
			continue
		}

		statusTopic := app.getTopicPrefix() + "/status/display_" + display.DisplayID + "_brightness"
		client.Publish(statusTopic, 0, true, strconv.Itoa(brightness))
	}
}

func (app *Application) messagePubHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received message: %s from topic: %s\n", msg.Payload(), msg.Topic())
	app.listen(client, msg)
}

func (app *Application) connectHandler(client mqtt.Client) {
	log.Println("Connected to MQTT")

	// Set up device configuration (in case this is a reconnection)
	app.setDevice(client)

	token := client.Publish(app.getTopicPrefix()+"/status/alive", 0, true, "online")
	token.Wait()

	log.Println("Sending 'online' to topic: " + app.getTopicPrefix() + "/status/alive")
	app.sub(client, app.getTopicPrefix()+"/command/#")

	// Start media stream if not already running (for reconnections)
	if macos.IsMediaControlAvailable() {
		go app.startMediaStream(client)
	}

	// Start user activity monitoring
	go app.startUserActivityMonitoring(client)

	// Send initial state updates
	app.updateVolume(client)
	app.updateMute(client)
	app.updateCaffeinateStatus(client)
	app.updateDisplayBrightness(client)
	app.updateNowPlaying(client)
	app.setUserActivityState(client, "inactive") // Initial state
}

func (app *Application) connectLostHandler(_ mqtt.Client, err error) {
	log.Printf("Disconnected from MQTT: %v", err)

	// Check if it's a network issue
	if !app.isNetworkReachable() {
		log.Println("MQTT broker is not reachable - likely on a different network")
		log.Println("Will retry connection when network becomes available")
	} else {
		log.Println("MQTT client will attempt to reconnect automatically...")
	}
}

func (app *Application) getMQTTClient() error {
	return app.getMQTTClientWithRetry(0)
}

// isNetworkReachable checks if the MQTT broker is reachable before attempting connection
func (app *Application) isNetworkReachable() bool {
	// Try to connect to the broker with a short timeout
	timeout := 5 * time.Second
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(app.config.IP, app.config.Port), timeout)
	if err != nil {
		log.Printf("Network check failed: MQTT broker %s:%s is not reachable (%v)", app.config.IP, app.config.Port, err)
		return false
	}
	conn.Close()
	return true
}

func (app *Application) getMQTTClientWithRetry(retryCount int) error {
	// Prevent infinite recursion
	if retryCount > MaxRetryAttempts {
		return fmt.Errorf("failed to connect to MQTT broker after multiple attempts")
	}

	// Check network reachability first to avoid long timeouts
	if !app.isNetworkReachable() {
		log.Printf("MQTT broker is not reachable on current network, will retry later")
		return fmt.Errorf("MQTT broker not reachable")
	}

	opts := mqtt.NewClientOptions()

	// Determine protocol and broker URL
	protocol := "tcp"
	if app.config.SSL {
		protocol = "ssl"
	}
	brokerURL := fmt.Sprintf("%s://%s:%s", protocol, app.config.IP, app.config.Port)
	log.Printf("Connecting to MQTT broker: %s", brokerURL)

	opts.AddBroker(brokerURL)
	if app.config.User != "" {
		opts.SetUsername(app.config.User)
	}
	if app.config.Password != "" {
		opts.SetPassword(app.config.Password)
	}

	// Set up handlers with application context
	opts.OnConnect = app.connectHandler
	opts.OnConnectionLost = app.connectLostHandler
	opts.SetDefaultPublishHandler(app.messagePubHandler)

	// Set client ID to ensure unique identification with timestamp to avoid conflicts
	clientID := fmt.Sprintf("%s_mac2mqtt_%d", app.hostname, time.Now().Unix())
	opts.SetClientID(clientID)

	// Network-aware connection reliability settings
	opts.SetClientID(app.hostname + "_mac2mqtt")
	opts.SetKeepAlive(60 * time.Second)      // Send ping every 60 seconds
	opts.SetPingTimeout(10 * time.Second)    // Shorter ping timeout for faster network change detection
	opts.SetConnectTimeout(15 * time.Second) // Shorter connect timeout for network switching
	opts.SetAutoReconnect(true)              // Enable auto-reconnect
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(15 * time.Second) // Wait 15 seconds between retries (good for network switches)
	opts.SetMaxReconnectInterval(2 * time.Minute)  // Max 2 minutes between reconnect attempts (faster recovery)
	opts.SetCleanSession(false)                    // Resume session to avoid losing subscriptions
	opts.SetOrderMatters(false)                    // Allow out-of-order delivery for better performance
	opts.SetWriteTimeout(10 * time.Second)         // Shorter write timeout for network issues
	opts.SetResumeSubs(true)                       // Resume subscriptions on reconnect

	// Set will message
	opts.SetWill(app.getTopicPrefix()+"/status/alive", "offline", 0, true)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		// If SSL connection fails, try falling back to non-SSL
		if app.config.SSL {
			log.Printf("SSL connection failed: %v. Trying non-SSL connection...", token.Error())
			app.config.SSL = false
			return app.getMQTTClientWithRetry(retryCount + 1)
		}
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}

	app.client = client
	return nil
}

func (app *Application) sub(client mqtt.Client, topic string) {
	token := client.Subscribe(topic, 0, nil)
	token.Wait()
	log.Printf("Subscribed to topic: %s\n", topic)
}

func (app *Application) listen(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// Handle volume commands
	if app.handleVolumeCommand(client, topic, payload) {
		return
	}

	// Handle mute commands
	if app.handleMuteCommand(client, topic, payload) {
		return
	}

	// Handle system commands
	if app.handleSystemCommand(topic, payload) {
		return
	}

	// Handle display brightness commands
	if app.handleDisplayBrightnessCommand(client, topic, payload) {
		return
	}

	// Handle shortcut commands
	if app.handleShortcutCommand(topic, payload) {
		return
	}

	// Handle keep awake commands
	if app.handleKeepAwakeCommand(client, topic, payload) {
		return
	}

	// Handle play/pause commands
	if app.handlePlayPauseCommand(client, topic, payload) {
		return
	}

	// Handle LM Studio commands
	if app.config.LMStudioEnabled {
		if app.handleLMStudioCommand(client, topic, payload) {
			return
		}
	}
}

// handleVolumeCommand handles volume control commands
func (app *Application) handleVolumeCommand(client mqtt.Client, topic, payload string) bool {
	if topic != app.getTopicPrefix()+"/command/volume" {
		return false
	}

	volume, err := app.validateVolumeInput(payload)
	if err != nil {
		log.Printf("Invalid volume value: %v", err)
		return true
	}

	macos.SetVolume(volume)
	app.updateVolume(client)
	app.updateMute(client)
	return true
}

// handleMuteCommand handles mute control commands
func (app *Application) handleMuteCommand(client mqtt.Client, topic, payload string) bool {
	if topic != app.getTopicPrefix()+"/command/mute" {
		return false
	}

	mute, err := app.validateMuteInput(payload)
	if err != nil {
		log.Printf("Invalid mute value: %v", err)
		return true
	}

	macos.SetMute(mute)
	app.updateVolume(client)
	app.updateMute(client)
	return true
}

// handleSystemCommand handles system control commands
func (app *Application) handleSystemCommand(topic, payload string) bool {
	if topic != app.getTopicPrefix()+"/command/set" {
		return false
	}

	switch payload {
	case "sleep":
		macos.Sleep()
	case "displaysleep":
		macos.DisplaySleep()
	case "displaywake":
		macos.DisplayWake()
	case "shutdown":
		macos.Shutdown()
	case "screensaver":
		macos.Screensaver()
	default:
		log.Printf("Unknown system command: %s", payload)
	}
	return true
}

// handleDisplayBrightnessCommand handles display brightness commands
func (app *Application) handleDisplayBrightnessCommand(client mqtt.Client, topic, payload string) bool {
	// Check if we have any displays available
	if len(app.displays) == 0 {
		log.Printf("Received display brightness command but no displays are available")
		log.Printf("Topic: %s, Payload: %s", topic, payload)
		log.Println("This usually means BetterDisplay CLI is not installed or not accessible")
		return true // Return true to indicate we handled the command
	}

	for _, display := range app.displays {
		commandTopic := app.getTopicPrefix() + "/command/display_" + display.DisplayID + "_brightness"
		if topic == commandTopic {
			brightness, err := app.validateBrightnessInput(payload)
			if err != nil {
				log.Printf("Invalid brightness value for display %s: %v", display.Name, err)
				return true
			}

			err = macos.SetDisplayBrightness(display.DisplayID, brightness)
			if err != nil {
				log.Printf("Error setting brightness for display %s: %v", display.Name, err)
				// Check if it's a BetterDisplay CLI error
				if !macos.IsBetterDisplayCLIAvailable() {
					log.Println("BetterDisplay CLI is not available. Please install BetterDisplay and enable CLI access.")
				}
			} else {
				// Update the status immediately
				statusTopic := app.getTopicPrefix() + "/status/display_" + display.DisplayID + "_brightness"
				client.Publish(statusTopic, 0, true, strconv.Itoa(brightness))
			}
			return true
		}
	}
	return false
}

// handleShortcutCommand handles shortcut execution commands
func (app *Application) handleShortcutCommand(topic, payload string) bool {
	if topic != app.getTopicPrefix()+"/command/runshortcut" {
		return false
	}

	if err := app.validateShortcutInput(payload); err != nil {
		log.Printf("Invalid shortcut: %v", err)
		return true
	}

	macos.RunShortcut(payload)
	return true
}

// handleKeepAwakeCommand handles keep awake commands
func (app *Application) handleKeepAwakeCommand(client mqtt.Client, topic, payload string) bool {
	if topic != app.getTopicPrefix()+"/command/keepawake" {
		return false
	}

	keepAwake, err := app.validateKeepAwakeInput(payload)
	if err != nil {
		log.Printf("Invalid keep awake value: %v", err)
		return true
	}

	if keepAwake {
		macos.KeepAwake()
	} else {
		macos.AllowSleep()
	}
	app.updateCaffeinateStatus(client)
	return true
}

// handlePlayPauseCommand handles play/pause commands
func (app *Application) handlePlayPauseCommand(client mqtt.Client, topic, payload string) bool {
	if topic != app.getTopicPrefix()+"/command/playpause" {
		return false
	}

	if payload == "playpause" {
		macos.PlayPause()
		// Update the now playing sensor after a short delay to reflect the new state
		time.Sleep(500 * time.Millisecond)
		app.updateNowPlaying(client)
	}
	return true
}

// handleLMStudioCommand handles LM Studio control commands
func (app *Application) handleLMStudioCommand(client mqtt.Client, topic, payload string) bool {
	basePrefix := app.getTopicPrefix()

	// Handle server start/stop
	if topic == basePrefix+"/command/lmstudio_server" {
		switch payload {
		case "start":
			if err := macos.StartLMStudioServer(); err != nil {
				log.Printf("Failed to start LM Studio server: %v", err)
			} else {
				log.Println("LM Studio server start command sent")
				// Wait a bit for the server to start and then update status
				time.Sleep(3 * time.Second)
				app.updateLMStudioStatus(client)
			}
		case "stop":
			if err := macos.StopLMStudioServer(); err != nil {
				log.Printf("Failed to stop LM Studio server: %v", err)
			} else {
				log.Println("LM Studio server stop command sent")
				// Wait a bit for the server to stop and then update status
				time.Sleep(2 * time.Second)
				app.updateLMStudioStatus(client)
			}
		default:
			log.Printf("Unknown LM Studio server command: %s", payload)
		}
		return true
	}

	// Handle model load
	if topic == basePrefix+"/command/lmstudio_load_model" {
		if payload == "" {
			log.Println("Empty model ID provided for load command")
			return true
		}

		if err := macos.LoadLMStudioModel(payload); err != nil {
			log.Printf("Failed to load model %s: %v", payload, err)
			client.Publish(basePrefix+"/status/lmstudio_last_error", 0, false, fmt.Sprintf("Failed to load model: %v", err))
		} else {
			log.Printf("Model %s load command sent", payload)
			// Wait a bit for the model to load and then update status
			time.Sleep(5 * time.Second)
			app.updateLMStudioStatus(client)
		}
		return true
	}

	// Handle model unload
	if topic == basePrefix+"/command/lmstudio_unload_model" {
		if payload == "" || payload == "all" {
			if err := macos.UnloadAllLMStudioModels(); err != nil {
				log.Printf("Failed to unload all models: %v", err)
			} else {
				log.Println("All models unload command sent")
				time.Sleep(2 * time.Second)
				app.updateLMStudioStatus(client)
			}
		} else {
			if err := macos.UnloadLMStudioModel(payload); err != nil {
				log.Printf("Failed to unload model %s: %v", payload, err)
			} else {
				log.Printf("Model %s unload command sent", payload)
				time.Sleep(2 * time.Second)
				app.updateLMStudioStatus(client)
			}
		}
		return true
	}

	return false
}

// updateLMStudioStatus updates the MQTT topics with current LM Studio status
func (app *Application) updateLMStudioStatus(client mqtt.Client) {
	if !app.config.LMStudioEnabled {
		return
	}

	basePrefix := app.getTopicPrefix()

	// Check if server is running
	isRunning, err := macos.GetLMStudioServerStatus(app.config.LMStudioAPIURL)
	if err != nil {
		log.Printf("Error checking LM Studio server status: %v", err)
		return
	}

	app.lmstudioMutex.Lock()
	app.lmstudioServerRunning = isRunning
	app.lmstudioMutex.Unlock()

	// Publish server status
	serverStatus := "offline"
	if isRunning {
		serverStatus = "online"
	}
	client.Publish(basePrefix+"/status/lmstudio_server", 0, false, serverStatus)

	if !isRunning {
		// Server is not running, clear model lists
		client.Publish(basePrefix+"/status/lmstudio_loaded_models", 0, false, "[]")
		client.Publish(basePrefix+"/status/lmstudio_available_models", 0, false, "[]")
		return
	}

	// Get all models
	models, err := macos.ListLMStudioModels(app.config.LMStudioAPIURL)
	if err != nil {
		log.Printf("Error listing LM Studio models: %v", err)
		return
	}

	// Separate loaded and available models
	var loadedModels []macos.LMStudioModel
	var availableModels []macos.LMStudioModel

	for _, model := range models {
		if model.State == "loaded" {
			loadedModels = append(loadedModels, model)
		} else {
			availableModels = append(availableModels, model)
		}
	}

	app.lmstudioMutex.Lock()
	app.lmstudioLoadedModels = loadedModels
	app.lmstudioMutex.Unlock()

	// Publish loaded models
	loadedJSON, _ := json.Marshal(loadedModels)
	client.Publish(basePrefix+"/status/lmstudio_loaded_models", 0, false, string(loadedJSON))

	// Publish available models
	availableJSON, _ := json.Marshal(availableModels)
	client.Publish(basePrefix+"/status/lmstudio_available_models", 0, false, string(availableJSON))

	// Publish model count
	client.Publish(basePrefix+"/status/lmstudio_loaded_models_count", 0, false, strconv.Itoa(len(loadedModels)))
	client.Publish(basePrefix+"/status/lmstudio_available_models_count", 0, false, strconv.Itoa(len(availableModels)))

	// Publish formatted model lists
	loadedList := macos.FormatModelList(loadedModels)
	availableList := macos.FormatModelList(availableModels)
	client.Publish(basePrefix+"/status/lmstudio_loaded_models_list", 0, false, loadedList)
	client.Publish(basePrefix+"/status/lmstudio_available_models_list", 0, false, availableList)

	log.Printf("LM Studio status updated: Server=%s, Loaded=%d, Available=%d", serverStatus, len(loadedModels), len(availableModels))
}


func (app *Application) updateVolume(client mqtt.Client) {
	token := client.Publish(app.getTopicPrefix()+"/status/volume", 0, false, strconv.Itoa(macos.GetVolume()))
	token.Wait()
}

func (app *Application) updateMute(client mqtt.Client) {
	token := client.Publish(app.getTopicPrefix()+"/status/mute", 0, false, strconv.FormatBool(macos.GetMuteStatus()))
	token.Wait()
}

// DiskUsage holds disk usage statistics
type DiskUsage = macos.DiskUsage
type CPUUsage = macos.CPUUsage
type MemoryUsage = macos.MemoryUsage
type UptimeInfo = macos.UptimeInfo

func (app *Application) getCPUUsage() (*CPUUsage, error) {
	cpu := sigar.Cpu{}
	if err := cpu.Get(); err != nil {
		return nil, fmt.Errorf("failed to get CPU stats: %w", err)
	}

	app.cpuMutex.Lock()
	defer app.cpuMutex.Unlock()

	// Calculate the delta since last measurement
	userDelta := cpu.User - app.lastCPU.User
	sysDelta := cpu.Sys - app.lastCPU.Sys
	idleDelta := cpu.Idle - app.lastCPU.Idle
	waitDelta := cpu.Wait - app.lastCPU.Wait
	niceDelta := cpu.Nice - app.lastCPU.Nice
	irqDelta := cpu.Irq - app.lastCPU.Irq
	softIrqDelta := cpu.SoftIrq - app.lastCPU.SoftIrq
	stolenDelta := cpu.Stolen - app.lastCPU.Stolen

	// Calculate total time delta
	totalDelta := userDelta + sysDelta + idleDelta + waitDelta + niceDelta + irqDelta + softIrqDelta + stolenDelta

	// Store current CPU stats for next calculation
	app.lastCPU = cpu

	// If this is the first measurement or total is zero, return 0% usage
	if totalDelta == 0 {
		return &CPUUsage{
			UsedPercent: 0,
			FreePercent: 100,
		}, nil
	}

	// Calculate idle and used percentages
	idlePercent := float64(idleDelta) / float64(totalDelta) * 100
	usedPercent := 100 - idlePercent

	return &CPUUsage{
		UsedPercent: usedPercent,
		FreePercent: idlePercent,
	}, nil
}

func (app *Application) updateBattery(client mqtt.Client) {
	token := client.Publish(app.getTopicPrefix()+"/status/battery", 0, false, macos.GetBatteryChargePercent())
	token.Wait()
}

func (app *Application) updateCaffeinateStatus(client mqtt.Client) {
	token := client.Publish(app.getTopicPrefix()+"/status/caffeinate", 0, false, strconv.FormatBool(macos.GetCaffeinateStatus()))
	token.Wait()
}

func (app *Application) updateDiskUsage(client mqtt.Client) {
	diskUsage, err := macos.GetDiskUsage()
	if err != nil {
		log.Printf("Failed to get disk usage: %v", err)
		return
	}

	// Publish individual metrics
	client.Publish(app.getTopicPrefix()+"/status/disk/total", 0, false, fmt.Sprintf("%d", diskUsage.Total))
	client.Publish(app.getTopicPrefix()+"/status/disk/used", 0, false, fmt.Sprintf("%d", diskUsage.Used))
	client.Publish(app.getTopicPrefix()+"/status/disk/free", 0, false, fmt.Sprintf("%d", diskUsage.Free))
	client.Publish(app.getTopicPrefix()+"/status/disk/used_percent", 0, false, fmt.Sprintf("%.2f", diskUsage.UsedPercent))
	client.Publish(app.getTopicPrefix()+"/status/disk/free_percent", 0, false, fmt.Sprintf("%.2f", diskUsage.FreePercent))
}

func (app *Application) updateCPUUsage(client mqtt.Client) {
	cpuUsage, err := app.getCPUUsage()
	if err != nil {
		log.Printf("Failed to get CPU usage: %v", err)
		return
	}

	// Publish CPU metrics
	client.Publish(app.getTopicPrefix()+"/status/cpu/used_percent", 0, false, fmt.Sprintf("%.2f", cpuUsage.UsedPercent))
	client.Publish(app.getTopicPrefix()+"/status/cpu/free_percent", 0, false, fmt.Sprintf("%.2f", cpuUsage.FreePercent))
}

func (app *Application) updateMemoryUsage(client mqtt.Client) {
	memUsage, err := macos.GetMemoryUsage()
	if err != nil {
		log.Printf("Failed to get memory usage: %v", err)
		return
	}

	// Publish memory metrics
	client.Publish(app.getTopicPrefix()+"/status/memory/total", 0, false, fmt.Sprintf("%d", memUsage.Total))
	client.Publish(app.getTopicPrefix()+"/status/memory/used", 0, false, fmt.Sprintf("%d", memUsage.Used))
	client.Publish(app.getTopicPrefix()+"/status/memory/free", 0, false, fmt.Sprintf("%d", memUsage.Free))
	client.Publish(app.getTopicPrefix()+"/status/memory/used_percent", 0, false, fmt.Sprintf("%.2f", memUsage.UsedPercent))
	client.Publish(app.getTopicPrefix()+"/status/memory/free_percent", 0, false, fmt.Sprintf("%.2f", memUsage.FreePercent))
}

func (app *Application) updateUptime(client mqtt.Client) {
	uptime, err := macos.GetSystemUptime()
	if err != nil {
		log.Printf("Failed to get uptime: %v", err)
		return
	}

	// Publish uptime metrics
	client.Publish(app.getTopicPrefix()+"/status/uptime/seconds", 0, false, fmt.Sprintf("%d", uptime.Seconds))
	client.Publish(app.getTopicPrefix()+"/status/uptime/human", 0, false, uptime.Human)
}

func (app *Application) updateMediaDevices(client mqtt.Client) {
	isMicOn, isCameraOn, err := macos.GetMediaDevicesState()
	if err != nil {
		log.Printf("Failed to get media devices state: %v", err)
		// Publish "unknown" state on error
		client.Publish(app.getTopicPrefix()+"/status/microphone", 0, false, "OFF")
		client.Publish(app.getTopicPrefix()+"/status/camera", 0, false, "OFF")
		return
	}

	// Publish media device states
	micState := "OFF"
	if isMicOn {
		micState = "ON"
	}
	cameraState := "OFF"
	if isCameraOn {
		cameraState = "ON"
	}

	client.Publish(app.getTopicPrefix()+"/status/microphone", 0, false, micState)
	client.Publish(app.getTopicPrefix()+"/status/camera", 0, false, cameraState)
}

func (app *Application) updatePublicIP(client mqtt.Client) {
	publicIP, err := macos.GetPublicIP()
	if err != nil {
		log.Printf("Failed to get public IP: %v", err)
		// Publish empty string on error
		client.Publish(app.getTopicPrefix()+"/status/public_ip", 0, false, "unavailable")
		return
	}

	// Publish public IP
	client.Publish(app.getTopicPrefix()+"/status/public_ip", 0, false, publicIP)
}

func (app *Application) setDevice(client mqtt.Client) {

	keepawake := map[string]interface{}{
		"p":             "switch",
		"name":          "Keep Awake",
		"unique_id":     app.hostname + "_keepwake",
		"command_topic": app.getTopicPrefix() + "/command/keepawake",
		"payload_on":    "true",
		"payload_off":   "false",
		"state_topic":   app.getTopicPrefix() + "/status/caffeinate",
		"icon":          "mdi:coffee",
	}

	displaywake := map[string]interface{}{
		"p":             "button",
		"name":          "Display Wake",
		"unique_id":     app.hostname + "_displaywake",
		"command_topic": app.getTopicPrefix() + "/command/set",
		"payload_press": "displaywake",
		"icon":          "mdi:monitor",
	}

	displaysleep := map[string]interface{}{
		"p":             "button",
		"name":          "Display Sleep",
		"unique_id":     app.hostname + "_displaysleep",
		"command_topic": app.getTopicPrefix() + "/command/set",
		"payload_press": "displaysleep",
		"icon":          "mdi:monitor-off",
	}

	screensaver := map[string]interface{}{
		"p":             "button",
		"name":          "Screensaver",
		"unique_id":     app.hostname + "_screensaver",
		"command_topic": app.getTopicPrefix() + "/command/set",
		"payload_press": "screensaver",
		"icon":          "mdi:monitor-star",
	}

	sleep := map[string]interface{}{
		"p":             "button",
		"name":          "Sleep",
		"unique_id":     app.hostname + "_sleep",
		"command_topic": app.getTopicPrefix() + "/command/set",
		"payload_press": "sleep",
		"icon":          "mdi:sleep",
	}

	shutdown := map[string]interface{}{
		"p":                  "button",
		"name":               "Shutdown",
		"unique_id":          app.hostname + "_shutdown",
		"command_topic":      app.getTopicPrefix() + "/command/set",
		"payload_press":      "shutdown",
		"enabled_by_default": false,
		"icon":               "mdi:power",
	}
	mute := map[string]interface{}{
		"p":             "switch",
		"name":          "Mute",
		"unique_id":     app.hostname + "_mute",
		"command_topic": app.getTopicPrefix() + "/command/mute",
		"payload_on":    "true",
		"payload_off":   "false",
		"state_topic":   app.getTopicPrefix() + "/status/mute",
		"icon":          "mdi:volume-mute",
	}

	volume := map[string]interface{}{
		"p":             "number",
		"name":          "Volume",
		"unique_id":     app.hostname + "_volume",
		"command_topic": app.getTopicPrefix() + "/command/volume",
		"state_topic":   app.getTopicPrefix() + "/status/volume",
		"min_value":     MinVolume,
		"max_value":     MaxVolume,
		"step":          1,
		"mode":          "slider",
		"icon":          "mdi:volume-high",
	}

	battery := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Battery",
		"unique_id":           app.hostname + "_battery",
		"state_topic":         app.getTopicPrefix() + "/status/battery",
		"enabled_by_default":  false,
		"unit_of_measurement": "%",
		"device_class":        "battery",
	}

	diskTotal := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Disk Total",
		"unique_id":           app.hostname + "_disk_total",
		"state_topic":         app.getTopicPrefix() + "/status/disk/total",
		"unit_of_measurement": "B",
		"device_class":        "data_size",
		"state_class":         "measurement",
		"icon":                "mdi:harddisk",
	}

	diskUsed := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Disk Used",
		"unique_id":           app.hostname + "_disk_used",
		"state_topic":         app.getTopicPrefix() + "/status/disk/used",
		"unit_of_measurement": "B",
		"device_class":        "data_size",
		"state_class":         "measurement",
		"icon":                "mdi:harddisk",
	}

	diskFree := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Disk Free",
		"unique_id":           app.hostname + "_disk_free",
		"state_topic":         app.getTopicPrefix() + "/status/disk/free",
		"unit_of_measurement": "B",
		"device_class":        "data_size",
		"state_class":         "measurement",
		"icon":                "mdi:harddisk",
	}

	diskUsedPercent := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Disk Used Percent",
		"unique_id":           app.hostname + "_disk_used_percent",
		"state_topic":         app.getTopicPrefix() + "/status/disk/used_percent",
		"unit_of_measurement": "%",
		"state_class":         "measurement",
		"icon":                "mdi:chart-pie",
	}

	diskFreePercent := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Disk Free Percent",
		"unique_id":           app.hostname + "_disk_free_percent",
		"state_topic":         app.getTopicPrefix() + "/status/disk/free_percent",
		"unit_of_measurement": "%",
		"state_class":         "measurement",
		"icon":                "mdi:chart-pie",
	}

	cpuUsedPercent := map[string]interface{}{
		"p":                   "sensor",
		"name":                "CPU Used Percent",
		"unique_id":           app.hostname + "_cpu_used_percent",
		"state_topic":         app.getTopicPrefix() + "/status/cpu/used_percent",
		"unit_of_measurement": "%",
		"state_class":         "measurement",
		"icon":                "mdi:cpu-64-bit",
	}

	cpuFreePercent := map[string]interface{}{
		"p":                   "sensor",
		"name":                "CPU Free Percent",
		"unique_id":           app.hostname + "_cpu_free_percent",
		"state_topic":         app.getTopicPrefix() + "/status/cpu/free_percent",
		"unit_of_measurement": "%",
		"state_class":         "measurement",
		"icon":                "mdi:cpu-64-bit",
	}

	memoryTotal := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Memory Total",
		"unique_id":           app.hostname + "_memory_total",
		"state_topic":         app.getTopicPrefix() + "/status/memory/total",
		"unit_of_measurement": "B",
		"device_class":        "data_size",
		"state_class":         "measurement",
		"icon":                "mdi:memory",
	}

	memoryUsed := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Memory Used",
		"unique_id":           app.hostname + "_memory_used",
		"state_topic":         app.getTopicPrefix() + "/status/memory/used",
		"unit_of_measurement": "B",
		"device_class":        "data_size",
		"state_class":         "measurement",
		"icon":                "mdi:memory",
	}

	memoryFree := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Memory Free",
		"unique_id":           app.hostname + "_memory_free",
		"state_topic":         app.getTopicPrefix() + "/status/memory/free",
		"unit_of_measurement": "B",
		"device_class":        "data_size",
		"state_class":         "measurement",
		"icon":                "mdi:memory",
	}

	memoryUsedPercent := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Memory Used Percent",
		"unique_id":           app.hostname + "_memory_used_percent",
		"state_topic":         app.getTopicPrefix() + "/status/memory/used_percent",
		"unit_of_measurement": "%",
		"state_class":         "measurement",
		"icon":                "mdi:memory",
	}

	memoryFreePercent := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Memory Free Percent",
		"unique_id":           app.hostname + "_memory_free_percent",
		"state_topic":         app.getTopicPrefix() + "/status/memory/free_percent",
		"unit_of_measurement": "%",
		"state_class":         "measurement",
		"icon":                "mdi:memory",
	}

	uptimeSeconds := map[string]interface{}{
		"p":                   "sensor",
		"name":                "Uptime Seconds",
		"unique_id":           app.hostname + "_uptime_seconds",
		"state_topic":         app.getTopicPrefix() + "/status/uptime/seconds",
		"unit_of_measurement": "s",
		"device_class":        "duration",
		"state_class":         "total_increasing",
		"icon":                "mdi:clock-outline",
	}

	uptimeHuman := map[string]interface{}{
		"p":           "sensor",
		"name":        "Uptime",
		"unique_id":   app.hostname + "_uptime_human",
		"state_topic": app.getTopicPrefix() + "/status/uptime/human",
		"icon":        "mdi:clock-outline",
	}

	microphone := map[string]interface{}{
		"p":            "binary_sensor",
		"name":         "Microphone",
		"unique_id":    app.hostname + "_microphone",
		"state_topic":  app.getTopicPrefix() + "/status/microphone",
		"payload_on":   "ON",
		"payload_off":  "OFF",
		"icon":         "mdi:microphone",
		"device_class": "running",
	}

	camera := map[string]interface{}{
		"p":            "binary_sensor",
		"name":         "Camera",
		"unique_id":    app.hostname + "_camera",
		"state_topic":  app.getTopicPrefix() + "/status/camera",
		"payload_on":   "ON",
		"payload_off":  "OFF",
		"icon":         "mdi:camera",
		"device_class": "running",
	}

	publicIP := map[string]interface{}{
		"p":           "sensor",
		"name":        "Public IP",
		"unique_id":   app.hostname + "_public_ip",
		"state_topic": app.getTopicPrefix() + "/status/public_ip",
		"icon":        "mdi:ip-network",
	}

	components := map[string]interface{}{
		"sleep":               sleep,
		"shutdown":            shutdown,
		"volume":              volume,
		"mute":                mute,
		"displaywake":         displaywake,
		"displaysleep":        displaysleep,
		"screensaver":         screensaver,
		"battery":             battery,
		"keepawake":           keepawake,
		"disk_total":          diskTotal,
		"disk_used":           diskUsed,
		"disk_free":           diskFree,
		"disk_used_percent":   diskUsedPercent,
		"disk_free_percent":   diskFreePercent,
		"cpu_used_percent":    cpuUsedPercent,
		"cpu_free_percent":    cpuFreePercent,
		"memory_total":        memoryTotal,
		"memory_used":         memoryUsed,
		"memory_free":         memoryFree,
		"memory_used_percent": memoryUsedPercent,
		"memory_free_percent": memoryFreePercent,
		"uptime_seconds":      uptimeSeconds,
		"uptime_human":        uptimeHuman,
		"microphone":          microphone,
		"camera":              camera,
		"public_ip":           publicIP,
	}

	// Add user activity sensor
	userActivity := map[string]interface{}{
		"p":            "binary_sensor",
		"name":         "User Activity",
		"unique_id":    app.hostname + "_user_activity",
		"state_topic":  app.getTopicPrefix() + "/status/user_activity",
		"payload_on":   "active",
		"payload_off":  "inactive",
		"icon":         "mdi:account-check",
		"device_class": "occupancy",
	}
	components["user_activity"] = userActivity

	// Add idle time sensor
	idleTime := map[string]interface{}{
		"p":                   "sensor",
		"name":                app.hostname + " User Idle Time",
		"unique_id":           app.hostname + "_idle_time_seconds",
		"state_topic":         app.getTopicPrefix() + "/status/idle_time_seconds",
		"unit_of_measurement": "s",
		"device_class":        "duration",
		"state_class":         "measurement",
		"icon":                "mdi:timer-sand",
	}
	components["idle_time_seconds"] = idleTime

	// Add media control components if Media Control is available
	if macos.IsMediaControlAvailable() {
		playPause := map[string]interface{}{
			"p":             "button",
			"name":          "Play/Pause",
			"unique_id":     app.hostname + "_playpause",
			"command_topic": app.getTopicPrefix() + "/command/playpause",
			"payload_press": "playpause",
			"icon":          "mdi:play-pause",
		}

		nowPlaying := map[string]interface{}{
			"p":                     "sensor",
			"name":                  "Now Playing",
			"unique_id":             app.hostname + "_now_playing",
			"state_topic":           app.getTopicPrefix() + "/status/now_playing",
			"json_attributes_topic": app.getTopicPrefix() + "/status/now_playing_attr",
			"icon":                  "mdi:music",
		}

		components["playpause"] = playPause
		components["now_playing"] = nowPlaying
	}

	// Note: Media player will be published as separate standard MQTT autodiscovery message

	// Add display brightness controls for each display
	for _, display := range app.displays {
		displayBrightness := map[string]interface{}{
			"p":             "number",
			"name":          display.Name + " Brightness",
			"unique_id":     app.hostname + "_display_" + display.DisplayID + "_brightness",
			"command_topic": app.getTopicPrefix() + "/command/display_" + display.DisplayID + "_brightness",
			"state_topic":   app.getTopicPrefix() + "/status/display_" + display.DisplayID + "_brightness",
			"min_value":     MinBrightness,
			"max_value":     MaxBrightness,
			"step":          1,
			"mode":          "slider",
			"icon":          "mdi:brightness-6",
		}
		components["display_"+display.DisplayID+"_brightness"] = displayBrightness
	}

	// Add LM Studio control components if enabled
	if app.config.LMStudioEnabled && macos.IsLMStudioCLIAvailable() {
		// Server control switch
		lmstudioServer := map[string]interface{}{
			"p":             "switch",
			"name":          "LM Studio Server",
			"unique_id":     app.hostname + "_lmstudio_server",
			"command_topic": app.getTopicPrefix() + "/command/lmstudio_server",
			"state_topic":   app.getTopicPrefix() + "/status/lmstudio_server",
			"payload_on":    "start",
			"payload_off":   "stop",
			"state_on":      "online",
			"state_off":     "offline",
			"icon":          "mdi:server",
		}
		components["lmstudio_server"] = lmstudioServer

		// Loaded models sensor
		lmstudioLoadedModels := map[string]interface{}{
			"p":           "sensor",
			"name":        "LM Studio Loaded Models",
			"unique_id":   app.hostname + "_lmstudio_loaded_models_list",
			"state_topic": app.getTopicPrefix() + "/status/lmstudio_loaded_models_list",
			"icon":        "mdi:brain",
		}
		components["lmstudio_loaded_models_list"] = lmstudioLoadedModels

		// Available models sensor
		lmstudioAvailableModels := map[string]interface{}{
			"p":           "sensor",
			"name":        "LM Studio Available Models",
			"unique_id":   app.hostname + "_lmstudio_available_models_list",
			"state_topic": app.getTopicPrefix() + "/status/lmstudio_available_models_list",
			"icon":        "mdi:database",
		}
		components["lmstudio_available_models_list"] = lmstudioAvailableModels

		// Loaded models count
		lmstudioLoadedCount := map[string]interface{}{
			"p":                   "sensor",
			"name":                "LM Studio Loaded Models Count",
			"unique_id":           app.hostname + "_lmstudio_loaded_models_count",
			"state_topic":         app.getTopicPrefix() + "/status/lmstudio_loaded_models_count",
			"unit_of_measurement": "models",
			"state_class":         "measurement",
			"icon":                "mdi:counter",
		}
		components["lmstudio_loaded_models_count"] = lmstudioLoadedCount

		// Load model text input (for manual model ID entry)
		lmstudioLoadModel := map[string]interface{}{
			"p":             "text",
			"name":          "LM Studio Load Model",
			"unique_id":     app.hostname + "_lmstudio_load_model",
			"command_topic": app.getTopicPrefix() + "/command/lmstudio_load_model",
			"icon":          "mdi:upload",
			"mode":          "text",
		}
		components["lmstudio_load_model"] = lmstudioLoadModel

		// Unload model text input (for manual model ID entry or "all")
		lmstudioUnloadModel := map[string]interface{}{
			"p":             "text",
			"name":          "LM Studio Unload Model",
			"unique_id":     app.hostname + "_lmstudio_unload_model",
			"command_topic": app.getTopicPrefix() + "/command/lmstudio_unload_model",
			"icon":          "mdi:download",
			"mode":          "text",
		}
		components["lmstudio_unload_model"] = lmstudioUnloadModel
	}

	origin := map[string]interface{}{
		"name": "mac2mqtt",
	}

	device := map[string]interface{}{
		"ids":  macos.GetSerialnumber(),
		"name": app.hostname,
		"mf":   "Apple",
		"mdl":  macos.GetModel(),
	}

	object := map[string]interface{}{
		"dev":                device,
		"o":                  origin,
		"cmps":               components,
		"availability_topic": app.getTopicPrefix() + "/status/alive",
		"qos":                2,
	}
	objectJSON, _ := json.Marshal(object)

	token := client.Publish(app.config.DiscoveryPrefix+"/device"+"/"+app.hostname+"/config", 0, true, objectJSON)
	token.Wait()

	// Note: Media player functionality replaced with play/pause button and now playing sensor
}

// handleOfflineMode manages application behavior when MQTT broker is unreachable
func (app *Application) handleOfflineMode() {
	log.Println("Operating in offline mode - MQTT broker not reachable")
	log.Println("Application will continue monitoring system state and attempt to reconnect periodically")

	// Continue basic system monitoring even when offline
	// This ensures the application doesn't crash and can recover when network returns
}

// Run starts the application and runs the main loop
func (app *Application) Run() error {
	log.Println("=== MAC2MQTT STARTING ===")
	log.Printf("Working directory: %s", macos.GetWorkingDirectory())
	log.Printf("Hostname set to: %s", app.hostname)
	log.Printf("Discovery Prefix: %s", app.config.DiscoveryPrefix)
	log.Printf("MQTT Broker: %s:%s", app.config.IP, app.config.Port)
	log.Printf("MQTT Topic: %s", app.topic)

	// Initialize displays before MQTT connection
	log.Println("=== DISCOVERING DISPLAYS ===")
	if len(app.displays) > 0 {
		log.Printf("Found %d display(s):", len(app.displays))
		for _, display := range app.displays {
			log.Printf("  - %s (ID: %s)", display.Name, display.DisplayID)
		}
	} else {
		log.Println("No displays found or BetterDisplay CLI not available")
	}
	log.Println("=== DISPLAY DISCOVERY COMPLETE ===")

	// Check Media Control availability
	log.Println("=== CHECKING MEDIA CONTROL ===")
	if macos.IsMediaControlAvailable() {
		log.Println("Media Control is available - Media player will be enabled")
	} else {
		log.Println("Media Control is not installed or not accessible")
		log.Println("To install Media Control:")
		log.Println("  1. Install via npm: npm install -g media-control")
		log.Println("  2. Or install via Homebrew: brew install media-control")
		log.Println("Media player information will be disabled until Media Control is available")
	}
	log.Println("=== MEDIA CONTROL CHECK COMPLETE ===")

	// Check LM Studio availability
	if app.config.LMStudioEnabled {
		log.Println("=== CHECKING LM STUDIO ===")
		if macos.IsLMStudioCLIAvailable() {
			log.Println("LM Studio CLI (lms) is available - LM Studio control will be enabled")
			log.Printf("LM Studio API URL: %s", app.config.LMStudioAPIURL)
		} else {
			log.Println("LM Studio CLI (lms) is not installed or not accessible")
			log.Println("To install LM Studio:")
			log.Println("  1. Download from https://lmstudio.ai/download")
			log.Println("  2. Run LM Studio at least once to install CLI tools")
			log.Println("LM Studio control will be disabled until CLI is available")
			app.config.LMStudioEnabled = false
		}
		log.Println("=== LM STUDIO CHECK COMPLETE ===")
	}

	log.Println("Starting MQTT connection...")
	if err := app.getMQTTClient(); err != nil {
		log.Printf("Initial MQTT connection failed: %v", err)
		if !app.isNetworkReachable() {
			log.Println("MQTT broker not reachable - starting in offline mode")
			app.handleOfflineMode()
			// Continue running, the network check ticker will handle reconnection
		} else {
			return fmt.Errorf("failed to connect to MQTT: %w", err)
		}
	}

	// Set up tickers for periodic updates
	volumeTicker := time.NewTicker(UpdateInterval)
	batteryTicker := time.NewTicker(UpdateInterval)
	awakeTicker := time.NewTicker(UpdateInterval)
	networkCheckTicker := time.NewTicker(30 * time.Second) // Check network every 30 seconds
	defer volumeTicker.Stop()
	defer batteryTicker.Stop()
	defer awakeTicker.Stop()
	defer networkCheckTicker.Stop()

	// Track connection state
	lastConnectionState := app.client.IsConnected()
	networkReachable := true

	// Initial setup - only if MQTT is connected
	if app.client != nil && app.client.IsConnected() {
		app.setDevice(app.client)
		app.updateVolume(app.client)
		app.updateMute(app.client)
		app.updateCaffeinateStatus(app.client)
		app.updateDisplayBrightness(app.client)
		app.updateNowPlaying(app.client)                 // Initial now playing update
		app.setUserActivityState(app.client, "inactive") // Initial user activity state
		app.updateDiskUsage(app.client)                  // Initial disk usage update
		app.updateCPUUsage(app.client)                   // Initial CPU usage update
		app.updateMemoryUsage(app.client)                // Initial memory usage update
		app.updateUptime(app.client)                     // Initial uptime update
		app.updateMediaDevices(app.client)               // Initial media devices update
		app.updatePublicIP(app.client)                   // Initial public IP update

		// Update LM Studio status if enabled
		if app.config.LMStudioEnabled {
			app.updateLMStudioStatus(app.client)
		}

		// Start media stream for real-time updates
		app.startMediaStream(app.client)

		// Start user activity monitoring
		app.startUserActivityMonitoring(app.client)
	} else {
		log.Println("Skipping initial MQTT setup - will configure when connection is established")
	}

	// Main event loop
	for {
		select {
		case <-volumeTicker.C:
			// Check if client is connected before publishing
			if app.client.IsConnected() {
				app.updateVolume(app.client)
				app.updateMute(app.client)
				app.updateMediaDevices(app.client)
				app.client.Publish(app.getTopicPrefix()+"/status/alive", 0, true, "online")
			} else if networkReachable {
				log.Println("MQTT client not connected but network is reachable, connection may be recovering")
			}

		case <-batteryTicker.C:
			if app.client.IsConnected() {
				app.updateBattery(app.client)
				app.updateDiskUsage(app.client)
				app.updateCPUUsage(app.client)
				app.updateMemoryUsage(app.client)
				app.updateUptime(app.client)
				app.updatePublicIP(app.client)
			} else if networkReachable {
				log.Println("MQTT client not connected but network is reachable, skipping battery update")
			}

		case <-awakeTicker.C:
			if app.client.IsConnected() {
				app.updateCaffeinateStatus(app.client)
				app.updateDisplayBrightness(app.client)

				// Update LM Studio status if enabled
				if app.config.LMStudioEnabled {
					app.updateLMStudioStatus(app.client)
				}
			} else if networkReachable {
				log.Println("MQTT client not connected but network is reachable, skipping status updates")
			}
			// Note: Media updates now come from the media-control stream

		case <-networkCheckTicker.C:
			// Periodic network reachability check
			currentNetworkState := app.isNetworkReachable()
			currentConnectionState := app.client.IsConnected()

			// Log network state changes
			if currentNetworkState != networkReachable {
				if currentNetworkState {
					log.Println("Network connectivity restored - MQTT broker is now reachable")
				} else {
					log.Println("Network connectivity lost - MQTT broker is no longer reachable")
				}
				networkReachable = currentNetworkState
			}

			// Log connection state changes
			if currentConnectionState != lastConnectionState {
				if currentConnectionState {
					log.Println("MQTT connection restored")
				} else {
					log.Println("MQTT connection lost")
				}
				lastConnectionState = currentConnectionState
			}

			// Handle network state changes
			if currentNetworkState && !networkReachable {
				// Network just became reachable - try to reconnect if not already connected
				if !currentConnectionState {
					log.Println("Attempting to reconnect to MQTT broker...")
					// The auto-reconnect should handle this, but we can force a reconnection attempt
					go func() {
						if token := app.client.Connect(); token.Wait() && token.Error() != nil {
							log.Printf("Reconnection attempt failed: %v", token.Error())
						}
					}()
				}
			}
		}
	}
}

// Input validation functions

// validateVolumeInput validates volume input (0-100)
func (app *Application) validateVolumeInput(payload string) (int, error) {
	volume, err := strconv.Atoi(payload)
	if err != nil {
		return 0, fmt.Errorf("volume must be a number: %w", err)
	}
	if volume < MinVolume || volume > MaxVolume {
		return 0, fmt.Errorf("volume must be between %d and %d, got %d", MinVolume, MaxVolume, volume)
	}
	return volume, nil
}

// validateMuteInput validates mute input (true/false)
func (app *Application) validateMuteInput(payload string) (bool, error) {
	mute, err := strconv.ParseBool(payload)
	if err != nil {
		return false, fmt.Errorf("mute must be true or false: %w", err)
	}
	return mute, nil
}

// validateBrightnessInput validates brightness input (0-100)
func (app *Application) validateBrightnessInput(payload string) (int, error) {
	brightness, err := strconv.Atoi(payload)
	if err != nil {
		return 0, fmt.Errorf("brightness must be a number: %w", err)
	}
	if brightness < MinBrightness || brightness > MaxBrightness {
		return 0, fmt.Errorf("brightness must be between %d and %d, got %d", MinBrightness, MaxBrightness, brightness)
	}
	return brightness, nil
}

// validateShortcutInput validates shortcut input
func (app *Application) validateShortcutInput(payload string) error {
	if payload == "" {
		return fmt.Errorf("shortcut name cannot be empty")
	}
	// Basic validation - shortcut name should be alphanumeric with spaces and hyphens
	matched, err := regexp.MatchString(`^[a-zA-Z0-9\s\-_]+$`, payload)
	if err != nil {
		return fmt.Errorf("error validating shortcut name: %w", err)
	}
	if !matched {
		return fmt.Errorf("shortcut name contains invalid characters")
	}
	return nil
}

// validateKeepAwakeInput validates keep awake input (true/false)
func (app *Application) validateKeepAwakeInput(payload string) (bool, error) {
	keepAwake, err := strconv.ParseBool(payload)
	if err != nil {
		return false, fmt.Errorf("keep awake must be true or false: %w", err)
	}
	return keepAwake, nil
}

func main() {
	// Create and initialize the application
	app, err := NewApplication()
	if err != nil {
		log.Fatal("Failed to initialize application: ", err)
	}

	// Run the application
	if err := app.Run(); err != nil {
		log.Fatal("Application error: ", err)
	}
}
