# LM Studio Integration for mac2mqtt

This integration allows you to control [LM Studio](https://lmstudio.ai) via MQTT and Home Assistant, enabling you to:

- Start/Stop the LM Studio server
- List available and loaded models
- Load and unload models
- Monitor server status and model information

## Prerequisites

1. **LM Studio** - Download and install from [https://lmstudio.ai/download](https://lmstudio.ai/download)
2. **LM Studio CLI (lms)** - Run LM Studio at least once to install the CLI tools
3. Verify the CLI is available by running `lms --help` in your terminal

## Configuration

Add the following to your `mac2mqtt.yaml` file:

```yaml
# LM Studio Integration
lmstudio_enabled: true
lmstudio_api_url: http://localhost:1234  # Default LM Studio API URL
```

### Configuration Options

- `lmstudio_enabled` (boolean): Enable or disable LM Studio integration
- `lmstudio_api_url` (string): The URL of the LM Studio REST API (default: `http://localhost:1234`)

## Home Assistant Entities

When LM Studio integration is enabled, the following entities will be created in Home Assistant:

### Switch
- **LM Studio Server** (`switch.HOSTNAME_lmstudio_server`)
  - Control: Turn on to start the server, off to stop it
  - State: Shows `online` when running, `offline` when stopped

### Sensors
- **LM Studio Loaded Models** (`sensor.HOSTNAME_lmstudio_loaded_models_list`)
  - Shows a human-readable list of currently loaded models

- **LM Studio Available Models** (`sensor.HOSTNAME_lmstudio_available_models_list`)
  - Shows a human-readable list of models available to load

- **LM Studio Loaded Models Count** (`sensor.HOSTNAME_lmstudio_loaded_models_count`)
  - Shows the number of currently loaded models

### Text Inputs
- **LM Studio Load Model** (`text.HOSTNAME_lmstudio_load_model`)
  - Enter a model ID to load it
  - Example: `meta-llama-3.1-8b-instruct` or `qwen2-vl-7b-instruct`

- **LM Studio Unload Model** (`text.HOSTNAME_lmstudio_unload_model`)
  - Enter a model ID to unload it, or `all` to unload all models
  - Example: `meta-llama-3.1-8b-instruct` or `all`

## MQTT Topics

### Command Topics
- `mac2mqtt/HOSTNAME/command/lmstudio_server` - Start or stop the server
  - Payload: `start` or `stop`

- `mac2mqtt/HOSTNAME/command/lmstudio_load_model` - Load a model
  - Payload: Model ID (e.g., `meta-llama-3.1-8b-instruct`)

- `mac2mqtt/HOSTNAME/command/lmstudio_unload_model` - Unload a model
  - Payload: Model ID or `all` to unload all models

### Status Topics
- `mac2mqtt/HOSTNAME/status/lmstudio_server` - Server status (`online` or `offline`)
- `mac2mqtt/HOSTNAME/status/lmstudio_loaded_models` - JSON array of loaded models
- `mac2mqtt/HOSTNAME/status/lmstudio_available_models` - JSON array of available models
- `mac2mqtt/HOSTNAME/status/lmstudio_loaded_models_count` - Number of loaded models
- `mac2mqtt/HOSTNAME/status/lmstudio_loaded_models_list` - Human-readable loaded models list
- `mac2mqtt/HOSTNAME/status/lmstudio_available_models_list` - Human-readable available models list

## Usage Examples

### Using MQTT Commands

```bash
# Start LM Studio server
mosquitto_pub -h YOUR_MQTT_BROKER -t "mac2mqtt/MacMini/command/lmstudio_server" -m "start"

# Stop LM Studio server
mosquitto_pub -h YOUR_MQTT_BROKER -t "mac2mqtt/MacMini/command/lmstudio_server" -m "stop"

# Load a model
mosquitto_pub -h YOUR_MQTT_BROKER -t "mac2mqtt/MacMini/command/lmstudio_load_model" -m "meta-llama-3.1-8b-instruct"

# Unload a specific model
mosquitto_pub -h YOUR_MQTT_BROKER -t "mac2mqtt/MacMini/command/lmstudio_unload_model" -m "meta-llama-3.1-8b-instruct"

# Unload all models
mosquitto_pub -h YOUR_MQTT_BROKER -t "mac2mqtt/MacMini/command/lmstudio_unload_model" -m "all"
```

### Home Assistant Automation Examples

#### Start LM Studio when you come home
```yaml
automation:
  - alias: "Start LM Studio when home"
    trigger:
      - platform: state
        entity_id: person.your_name
        to: "home"
    action:
      - service: switch.turn_on
        target:
          entity_id: switch.macmini_lmstudio_server
```

#### Load a model at a specific time
```yaml
automation:
  - alias: "Load AI model in the morning"
    trigger:
      - platform: time
        at: "08:00:00"
    condition:
      - condition: state
        entity_id: switch.macmini_lmstudio_server
        state: "on"
    action:
      - service: text.set_value
        target:
          entity_id: text.macmini_lmstudio_load_model
        data:
          value: "meta-llama-3.1-8b-instruct"
```

#### Unload all models when leaving
```yaml
automation:
  - alias: "Unload models when away"
    trigger:
      - platform: state
        entity_id: person.your_name
        to: "not_home"
    action:
      - service: text.set_value
        target:
          entity_id: text.macmini_lmstudio_unload_model
        data:
          value: "all"
```

## Model Information

The integration provides detailed model information including:
- **ID**: Model identifier (used for load/unload commands)
- **Type**: Model type (llm, vlm, embeddings)
- **Publisher**: Model publisher
- **Architecture**: Model architecture
- **Quantization**: Quantization level (e.g., Q4_K_M)
- **State**: Current state (loaded or not-loaded)
- **Max Context Length**: Maximum context window size

## Troubleshooting

### LM Studio CLI not found
If you see "lms CLI is not installed or not accessible":
1. Make sure LM Studio is installed
2. Run LM Studio at least once
3. Try running `lms --help` in your terminal
4. Add the LM Studio CLI path to your PATH if needed

### Server won't start
1. Check if LM Studio is already running (close the GUI if it is)
2. Check the port 1234 is not in use by another application
3. Review the mac2mqtt logs for error messages

### Models won't load
1. Make sure the model is downloaded in LM Studio
2. Check the model ID matches exactly (case-sensitive)
3. Ensure you have enough RAM/VRAM for the model
4. Check the LM Studio logs for detailed error messages

### API connection issues
1. Verify the `lmstudio_api_url` in your config matches the server URL
2. Check if the LM Studio server is actually running
3. Test the API directly: `curl http://localhost:1234/api/v0/models`

## Advanced Usage

### Loading Models with Options
While not directly exposed in Home Assistant, you can use the `lms` CLI directly:

```bash
# Load with specific GPU offload (0.0-1.0)
lms load meta-llama-3.1-8b-instruct --gpu=0.8

# Load with custom context length
lms load meta-llama-3.1-8b-instruct --context-length=8192

# Load with both options
lms load meta-llama-3.1-8b-instruct --gpu=1.0 --context-length=4096
```

### Monitoring via MQTT
Subscribe to all LM Studio status topics:

```bash
mosquitto_sub -h YOUR_MQTT_BROKER -t "mac2mqtt/+/status/lmstudio_#" -v
```

## References

- [LM Studio Documentation](https://lmstudio.ai/docs)
- [LM Studio CLI Documentation](https://lmstudio.ai/docs/cli)
- [LM Studio REST API](https://lmstudio.ai/docs/api/rest-api)
- [mac2mqtt GitHub](https://github.com/your-repo/mac2mqtt)

## Contributing

Found a bug or have a feature request? Please open an issue on GitHub!
