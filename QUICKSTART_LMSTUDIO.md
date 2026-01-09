# LM Studio Integration - Quick Start Guide

## What Was Implemented

I've successfully integrated LM Studio control into your mac2mqtt application! You can now:

✅ **Start/Stop LM Studio server** via MQTT and Home Assistant
✅ **List all available models** (downloaded in LM Studio)
✅ **Load models** by their ID
✅ **Unload models** individually or all at once
✅ **Monitor server status** and loaded models in real-time

## How to Use It

### 1. Enable LM Studio Integration

Edit your `mac2mqtt.yaml` file and add:

```yaml
lmstudio_enabled: true
lmstudio_api_url: http://localhost:1234  # Optional, this is the default
```

### 2. Rebuild and Restart mac2mqtt

```bash
# Build the updated application
go build -o mac2mqtt mac2mqtt.go

# Restart mac2mqtt (if running as a service)
./status.sh  # Check if running
# Then stop and start it according to your setup
```

### 3. In Home Assistant

After restarting, you'll see these new entities automatically (via MQTT discovery):

- **Switch**: `switch.HOSTNAME_lmstudio_server` - Start/stop the server
- **Sensor**: `sensor.HOSTNAME_lmstudio_loaded_models_list` - Shows loaded models
- **Sensor**: `sensor.HOSTNAME_lmstudio_available_models_list` - Shows available models
- **Sensor**: `sensor.HOSTNAME_lmstudio_loaded_models_count` - Number of loaded models
- **Text**: `text.HOSTNAME_lmstudio_load_model` - Enter model ID to load
- **Text**: `text.HOSTNAME_lmstudio_unload_model` - Enter model ID to unload

### 4. Quick Test via MQTT

```bash
# Start the server
mosquitto_pub -h YOUR_MQTT_BROKER -t "mac2mqtt/YOUR_HOSTNAME/command/lmstudio_server" -m "start"

# Load a model (replace with your model ID)
mosquitto_pub -h YOUR_MQTT_BROKER -t "mac2mqtt/YOUR_HOSTNAME/command/lmstudio_load_model" -m "meta-llama-3.1-8b-instruct"

# Check status
mosquitto_sub -h YOUR_MQTT_BROKER -t "mac2mqtt/YOUR_HOSTNAME/status/lmstudio_#" -v
```

## File Changes Summary

### New Files
1. **`macos/lmstudio.go`** - Core LM Studio integration module
2. **`LMSTUDIO_INTEGRATION.md`** - Comprehensive documentation
3. **`IMPLEMENTATION_SUMMARY.md`** - Technical implementation details

### Modified Files
1. **`mac2mqtt.go`** - Added LM Studio handling and status updates
2. **`config/config.go`** - Added configuration options
3. **`mac2mqtt.yaml`** - Added example configuration
4. **`README.md`** - Added LM Studio to feature list

## Prerequisites

Before using LM Studio integration, make sure you have:

1. **LM Studio installed**: Download from https://lmstudio.ai/download
2. **Run LM Studio once**: This installs the `lms` CLI tools
3. **Verify CLI**: Run `lms --help` in terminal to confirm

If the CLI isn't available, mac2mqtt will log a warning and disable the integration.

## Example Home Assistant Automation

Start LM Studio when you say "start AI":

```yaml
automation:
  - alias: "Voice command to start LM Studio"
    trigger:
      - platform: conversation
        command: "start AI"
    action:
      - service: switch.turn_on
        target:
          entity_id: switch.macmini_lmstudio_server
      - service: notify.mobile_app
        data:
          message: "Starting LM Studio server..."
```

## Troubleshooting

### "lms CLI is not installed"
- Install LM Studio and run it at least once
- Verify `lms --help` works in terminal

### "Server won't start"
- Check if LM Studio GUI is running (close it)
- Verify port 1234 is available

### "Models won't load"
- Ensure models are downloaded in LM Studio
- Check exact model ID spelling (case-sensitive)
- Verify sufficient RAM/VRAM

## Documentation

For detailed information, see:
- **[LMSTUDIO_INTEGRATION.md](LMSTUDIO_INTEGRATION.md)** - Complete usage guide
- **[IMPLEMENTATION_SUMMARY.md](IMPLEMENTATION_SUMMARY.md)** - Technical details

## What's Next?

The integration is fully functional! To test it:

1. Enable it in your config
2. Rebuild and restart mac2mqtt
3. Check Home Assistant for new entities
4. Try controlling the server and loading a model

Everything follows the existing patterns in your codebase, so it should integrate smoothly. The integration is optional and won't affect anything if you keep it disabled.

Enjoy controlling LM Studio from Home Assistant! 🚀
