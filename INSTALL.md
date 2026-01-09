# Mac2MQTT Installation Guide

This guide explains how to install Mac2MQTT on your macOS system.

## Quick Installation

The easiest way to install Mac2MQTT is using the provided installer script:

```bash
# Make sure you're in the project directory
./install.sh
```

The installer will:
1. Check system requirements (macOS, Go)
2. Build the Mac2MQTT binary
3. Configure MQTT settings interactively
4. Install optional dependencies (BetterDisplay CLI, Media Control)
5. Set up the service to run automatically
6. Create management scripts

## Prerequisites

- macOS (required)
- Go programming language (will be installed automatically if missing)
- MQTT broker (e.g., Home Assistant, Mosquitto)

## Optional Dependencies

For full functionality, consider installing:

### BetterDisplay CLI
Required for display brightness control:
1. Install BetterDisplay from https://github.com/waydabber/BetterDisplay
2. Enable CLI access in BetterDisplay settings

### Media Control
Required for media player information:
```bash
# Via npm
npm install -g media-control

# Or via Homebrew
brew install media-control
```

### LM Studio CLI
Required for AI model control via MQTT:
1. Download LM Studio from https://lmstudio.ai/download
2. Run LM Studio at least once to install CLI tools
3. The CLI will be automatically available at `~/.lmstudio/bin/lms`
4. Enable in `mac2mqtt.yaml`: `lmstudio_enabled: true`

**Note:** The installer automatically detects and configures LM Studio if installed.

## Manual Installation

If you prefer to install manually:

1. **Build the application:**
   ```bash
   go mod download
   go build -o mac2mqtt mac2mqtt.go
   chmod +x mac2mqtt
   ```

2. **Configure MQTT settings:**
   Edit `mac2mqtt.yaml` with your MQTT broker details.

3. **Create installation directory:**
   ```bash
   mkdir -p ~/mac2mqtt
   cp mac2mqtt mac2mqtt.yaml ~/mac2mqtt/
   ```

4. **Set up launch agent:**
   ```bash
   # The plist needs to use your actual $HOME path
   # Build the path dynamically
   cat > /tmp/com.hagak.mac2mqtt.plist << EOF
   <?xml version="1.0" encoding="UTF-8"?>
   <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
   <plist version="1.0">
       <dict>
           <key>Label</key>
           <string>com.hagak.mac2mqtt</string>
           <key>Program</key>
           <string>$HOME/mac2mqtt/mac2mqtt</string>
           <key>WorkingDirectory</key>
           <string>$HOME/mac2mqtt/</string>
           <key>RunAtLoad</key>
           <true/>
           <key>KeepAlive</key>
           <true/>
           <key>StandardErrorPath</key>
           <string>/tmp/mac2mqtt.job.err</string>
           <key>StandardOutPath</key>
           <string>/tmp/mac2mqtt.job.out</string>
           <key>EnvironmentVariables</key>
           <dict>
               <key>PATH</key>
               <string>$HOME/.lmstudio/bin:$HOME/.nvm/versions/node/current/bin:/usr/local/bin:/opt/homebrew/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
               <key>HOME</key>
               <string>$HOME</string>
               <key>USER</key>
               <string>$(whoami)</string>
               <key>LOGNAME</key>
               <string>$(whoami)</string>
               <key>SHELL</key>
               <string>/bin/zsh</string>
           </dict>
           <key>ProcessType</key>
           <string>Background</string>
           <key>ThrottleInterval</key>
           <integer>10</integer>
Common issues:
- **Wrong PATH in plist:** Ensure `$HOME` is correctly expanded, especially for non-standard home directories
- **Missing dependencies:** Check that LM Studio CLI (`~/.lmstudio/bin/lms`) and media-control are in PATH

### MQTT connection issues
1. Verify your MQTT broker is running
2. Check the configuration in `~/mac2mqtt/mac2mqtt.yaml`
3. Ensure network connectivity to the MQTT broker
4. The app will run in offline mode if MQTT is unreachable and retry automatically

### LM Studio not working
1. Verify LM Studio is installed and CLI tools are available:
   ```bash
   ls -la ~/.lmstudio/bin/lms
   ```
2. Check that the PATH in the plist includes `$HOME/.lmstudio/bin`
3. Restart the service after installing LM Studio:
   ```bash
   sudo launchctl kickstart -k gui/$(id -u)/com.hagak.mac2mqtt
   ```

### Permission issues
If you encounter permission issues:
1. The service should be installed in `/Library/LaunchAgents/` (system-wide, requires sudo)
2. Check file permissions: `ls -la /Library/LaunchAgents/com.hagak.mac2mqtt.plist`
3. Verify the binary is executable: `ls -la ~/mac2mqtt/mac2mqtt`

### Path issues
If you have a non-standard home directory (e.g., `/Volumes/...`):
1. Ensure all paths in the plist use the actual `$HOME` value, not `/Users/username`
2. Update the plist file manually if needed:
   ```bash
   sudo sed -i '' "s|/Users/$(whoami)|$HOME|g" /Library/LaunchAgents/com.hagak.mac2mqtt.plist
   sudo launchctl bootout gui/$(id -u)/com.hagak.mac2mqtt
   sudo launchctl bootstrap gui/$(id -u) /Library/LaunchAgents/com.hagak.mac2mqtt.plist
   ```
   sudo launchctl bootstrap gui/$(id -u) /Library/LaunchAgents/com.hagak.mac2mqtt.plist
   ```

   **Important:** The plist must include:
   - Correct `$HOME` path (especially for non-standard locations like `/Volumes/HubDrive/Users/...`)
   - LM Studio CLI path: `$HOME/.lmstudio/bin` in PATH
   - Node.js path if using media-control via npm

## Management

After installation, you can manage Mac2MQTT using:

### Using the Makefile (from project directory)
```bash
make status       # Check service status
make logs         # Show recent logs
make configure    # Reconfigure settings
make uninstall    # Uninstall mac2mqtt
```

### Using management scripts (from installation directory)
```bash
cd ~/mac2mqtt
./status.sh          # Check service status
./status.sh --logs   # Show recent logs
./status.sh --follow # Follow logs in real-time
./configure.sh       # Reconfigure MQTT settings
./uninstall.sh       # Uninstall Mac2MQTT
```
cd ~/mac2mqtt
./uninstall.sh

# Or using make
make uninstall

# Or manually
sudo launchctl bootout gui/$(id -u)/com.hagak.mac2mqtt
sudo rm /Library/LaunchAgents/com.hagak.mac2mqtt.plist
rm -rf ~/mac2mqtt
rm -f /tmp/mac2mqtt.job.{out,err}
```

This will:
- Stop the service
- Remove the launch agent
- Delete installation files
- Clean up log files

## Advanced Configuration

### Environment Variables in LaunchAgent
The service runs with these environment variables:
- `PATH`: Includes LM Studio CLI, Node.js, Homebrew, and system paths
- `HOME`: Your home directory (correctly handles non-standard locations)
- `USER`, `LOGNAME`: Your username
- `SHELL`: `/bin/zsh`

### Managing the Service
```bash
# Check if running
launchctl list | grep mac2mqtt

# View service details
launchctl print gui/$(id -u)/com.hagak.mac2mqtt

# Restart service
sudo launchctl kickstart -k gui/$(id -u)/com.hagak.mac2mqtt

# Stop service
sudo launchctl bootout gui/$(id -u)/com.hagak.mac2mqtt

# Start service
sudo launchctl bootstrap gui/$(id -u) /Library/LaunchAgents/com.hagak.mac2mqtt.plist
```broker is running
2. Check the configuration in `~/mac2mqtt/mac2mqtt.yaml`
3. Ensure network connectivity to the MQTT broker

### Permission issues
If you encounter permission issues, ensure:
1. You're not running as root
2. The launch agent has proper permissions
3. The installation directory is owned by your user

## Uninstallation

To completely remove Mac2MQTT:

```bash
# Using the uninstall script
./uninstall.sh

# Or using make
make uninstall
```

This will:
- Stop the service
- Remove the launch agent
- Delete installation files
- Clean up log files

## Support

For issues and questions:
1. Check the logs using `./status.sh --logs`
2. Review the README.md for configuration examples
3. Ensure all dependencies are properly installed