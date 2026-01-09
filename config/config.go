package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

// Config holds all configuration settings for mac2mqtt
type Config struct {
	IP               string `yaml:"mqtt_ip"`
	Port             string `yaml:"mqtt_port"`
	User             string `yaml:"mqtt_user"`
	Password         string `yaml:"mqtt_password"`
	SSL              bool   `yaml:"mqtt_ssl"`
	Hostname         string `yaml:"hostname"`
	Topic            string `yaml:"mqtt_topic"`
	DiscoveryPrefix  string `yaml:"discovery_prefix"`
	IdleActivityTime int    `yaml:"idle_activity_time"` // in seconds
}

// LoadConfig loads the configuration from mac2mqtt.yaml in the executable directory
func LoadConfig() (*Config, error) {
	c := &Config{}

	ex, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("failed to get executable path: %w", err)
	}
	exPath := filepath.Dir(ex)

	log.Printf("Path: %v", exPath)
	configContent, err := os.ReadFile(exPath + "/mac2mqtt.yaml")
	if err != nil {
		return nil, fmt.Errorf("no config file provided: %w", err)
	}

	err = yaml.Unmarshal(configContent, c)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults
	if c.IdleActivityTime == 0 {
		log.Println("No idle_activity_time specified in config, using default 10 seconds")
	}
	if c.DiscoveryPrefix == "" {
		c.DiscoveryPrefix = "homeassistant"
	}

	// Validate required fields
	if err := c.Validate(); err != nil {
		return nil, err
	}

	return c, nil
}

// Validate checks if all required configuration fields are set
func (c *Config) Validate() error {
	if c.IP == "" {
		return fmt.Errorf("mqtt_ip is required in mac2mqtt.yaml")
	}
	if c.Port == "" {
		return fmt.Errorf("mqtt_port is required in mac2mqtt.yaml")
	}
	return nil
}
