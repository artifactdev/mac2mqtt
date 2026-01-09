package macos

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// LMStudioModel represents a model available or loaded in LM Studio
type LMStudioModel struct {
	ID                 string `json:"id"`
	Object             string `json:"object"`
	Type               string `json:"type"`
	Publisher          string `json:"publisher"`
	Arch               string `json:"arch"`
	CompatibilityType  string `json:"compatibility_type"`
	Quantization       string `json:"quantization"`
	State              string `json:"state"` // "loaded" or "not-loaded"
	MaxContextLength   int    `json:"max_context_length"`
}

// LMStudioServerStatus represents the server status
type LMStudioServerStatus struct {
	IsRunning     bool
	LoadedModels  []LMStudioModel
	AvailableModels []LMStudioModel
}

// IsLMStudioCLIAvailable checks if lms CLI is installed and accessible
func IsLMStudioCLIAvailable() bool {
	_, err := exec.LookPath("lms")
	return err == nil
}

// StartLMStudioServer starts the LM Studio server
func StartLMStudioServer() error {
	if !IsLMStudioCLIAvailable() {
		return fmt.Errorf("lms CLI is not installed or not accessible")
	}

	log.Println("Starting LM Studio server...")
	cmd := exec.Command("lms", "server", "start")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start LM Studio server: %v, output: %s", err, string(output))
	}

	log.Printf("LM Studio server start command executed: %s", string(output))
	return nil
}

// StopLMStudioServer stops the LM Studio server
func StopLMStudioServer() error {
	if !IsLMStudioCLIAvailable() {
		return fmt.Errorf("lms CLI is not installed or not accessible")
	}

	log.Println("Stopping LM Studio server...")
	cmd := exec.Command("lms", "server", "stop")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop LM Studio server: %v, output: %s", err, string(output))
	}

	log.Printf("LM Studio server stop command executed: %s", string(output))
	return nil
}

// GetLMStudioServerStatus checks if the LM Studio server is running
func GetLMStudioServerStatus(apiURL string) (bool, error) {
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Get(apiURL + "/api/v0/models")
	if err != nil {
		return false, nil // Server not running or not reachable
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// ListLMStudioModels lists all available models from LM Studio API
func ListLMStudioModels(apiURL string) ([]LMStudioModel, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(apiURL + "/api/v0/models")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LM Studio API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var result struct {
		Object string            `json:"object"`
		Data   []LMStudioModel   `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return result.Data, nil
}

// GetLoadedModels returns a list of currently loaded models
func GetLoadedModels(apiURL string) ([]LMStudioModel, error) {
	models, err := ListLMStudioModels(apiURL)
	if err != nil {
		return nil, err
	}

	var loadedModels []LMStudioModel
	for _, model := range models {
		if model.State == "loaded" {
			loadedModels = append(loadedModels, model)
		}
	}

	return loadedModels, nil
}

// GetAvailableModels returns a list of models that are not loaded
func GetAvailableModels(apiURL string) ([]LMStudioModel, error) {
	models, err := ListLMStudioModels(apiURL)
	if err != nil {
		return nil, err
	}

	var availableModels []LMStudioModel
	for _, model := range models {
		if model.State == "not-loaded" {
			availableModels = append(availableModels, model)
		}
	}

	return availableModels, nil
}

// LoadLMStudioModel loads a model using the lms CLI
func LoadLMStudioModel(modelID string) error {
	if !IsLMStudioCLIAvailable() {
		return fmt.Errorf("lms CLI is not installed or not accessible")
	}

	log.Printf("Loading LM Studio model: %s", modelID)
	cmd := exec.Command("lms", "load", modelID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load model %s: %v, output: %s", modelID, err, string(output))
	}

	log.Printf("Model %s loaded successfully: %s", modelID, string(output))
	return nil
}

// UnloadLMStudioModel unloads a model using the lms CLI
func UnloadLMStudioModel(modelID string) error {
	if !IsLMStudioCLIAvailable() {
		return fmt.Errorf("lms CLI is not installed or not accessible")
	}

	log.Printf("Unloading LM Studio model: %s", modelID)
	cmd := exec.Command("lms", "unload", modelID)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to unload model %s: %v, output: %s", modelID, err, string(output))
	}

	log.Printf("Model %s unloaded successfully: %s", modelID, string(output))
	return nil
}

// UnloadAllLMStudioModels unloads all loaded models
func UnloadAllLMStudioModels() error {
	if !IsLMStudioCLIAvailable() {
		return fmt.Errorf("lms CLI is not installed or not accessible")
	}

	log.Println("Unloading all LM Studio models...")
	cmd := exec.Command("lms", "unload", "--all")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to unload all models: %v, output: %s", err, string(output))
	}

	log.Printf("All models unloaded successfully: %s", string(output))
	return nil
}

// GetLMStudioModelInfo gets detailed information about a specific model
func GetLMStudioModelInfo(apiURL, modelID string) (*LMStudioModel, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Get(apiURL + "/api/v0/models/" + modelID)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to LM Studio API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LM Studio API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	var model LMStudioModel
	if err := json.Unmarshal(body, &model); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	return &model, nil
}

// LoadLMStudioModelWithOptions loads a model with specific options using the lms CLI
func LoadLMStudioModelWithOptions(modelID string, gpuOffload float64, contextLength int) error {
	if !IsLMStudioCLIAvailable() {
		return fmt.Errorf("lms CLI is not installed or not accessible")
	}

	args := []string{"load", modelID}

	if gpuOffload > 0 {
		args = append(args, fmt.Sprintf("--gpu=%.2f", gpuOffload))
	}

	if contextLength > 0 {
		args = append(args, fmt.Sprintf("--context-length=%d", contextLength))
	}

	log.Printf("Loading LM Studio model: %s with options: gpu=%.2f, context-length=%d", modelID, gpuOffload, contextLength)
	cmd := exec.Command("lms", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to load model %s: %v, output: %s", modelID, err, string(output))
	}

	log.Printf("Model %s loaded successfully with options: %s", modelID, string(output))
	return nil
}

// ChatWithModel sends a chat completion request to a loaded model
func ChatWithModel(apiURL, modelID, userMessage string) (string, error) {
	client := &http.Client{
		Timeout: 120 * time.Second, // Longer timeout for inference
	}

	requestBody := map[string]interface{}{
		"model": modelID,
		"messages": []map[string]string{
			{"role": "user", "content": userMessage},
		},
		"temperature": 0.7,
		"max_tokens":  -1,
		"stream":      false,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %v", err)
	}

	resp, err := client.Post(apiURL+"/api/v0/chat/completions", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %v", err)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %v", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no response from model")
	}

	return result.Choices[0].Message.Content, nil
}

// GetServerStatusDetailed returns detailed status about LM Studio server and models
func GetServerStatusDetailed(apiURL string) (*LMStudioServerStatus, error) {
	status := &LMStudioServerStatus{}

	// Check if server is running
	isRunning, err := GetLMStudioServerStatus(apiURL)
	if err != nil {
		return nil, err
	}
	status.IsRunning = isRunning

	if !isRunning {
		return status, nil
	}

	// Get all models
	models, err := ListLMStudioModels(apiURL)
	if err != nil {
		return nil, err
	}

	// Separate loaded and available models
	for _, model := range models {
		if model.State == "loaded" {
			status.LoadedModels = append(status.LoadedModels, model)
		} else {
			status.AvailableModels = append(status.AvailableModels, model)
		}
	}

	return status, nil
}

// FormatModelList formats a list of models into a readable string
func FormatModelList(models []LMStudioModel) string {
	if len(models) == 0 {
		return "No models"
	}

	var parts []string
	for _, model := range models {
		parts = append(parts, fmt.Sprintf("%s (%s, %s)", model.ID, model.Type, model.State))
	}
	// Use newlines for better display in Home Assistant
	return strings.Join(parts, "\n")
}
