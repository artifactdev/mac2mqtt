# Refactoring-Plan für mac2mqtt

## Ziel
Aufteilung des monolithischen mac2mqtt.go (2410 Zeilen) in modulare Packages für bessere Wartbarkeit und Testbarkeit.

## Neue Struktur

```
mac2mqtt/
├── config/
│   └── config.go          # Konfigurationsmanagement
├── macos/
│   ├── system.go          # Systemkommandos (Sleep, Shutdown, etc.)
│   ├── audio.go           # Audio-Steuerung (Volume, Mute)
│   ├── display.go         # Display-Steuerung (Brightness)
│   ├── media.go           # Media-Kontrolle (Play/Pause, Info)
│   └── monitoring.go      # System-Monitoring (CPU, RAM, Disk, etc.)
├── mqtt/
│   ├── client.go          # MQTT-Client-Verwaltung
│   ├── handlers.go        # Message-Handler
│   └── discovery.go       # Home Assistant Discovery
├── main.go                # Haupteinstiegspunkt
└── mac2mqtt_test.go       # Tests

```

## Aufteilung nach Packages

### config/config.go
**Verantwortlichkeit:** Laden und Validieren der Konfiguration

**Funktionen:**
- `type Config struct` - Konfigurationsstruktur
- `LoadConfig() (*Config, error)` - Lädt die Konfiguration aus mac2mqtt.yaml
- `Validate() error` - Validiert die Konfiguration

**Aktueller Code:** Zeilen 119-177 in mac2mqtt.go

### macos/system.go
**Verantwortlichkeit:** Grundlegende Systemkommandos

**Funktionen:**
- `GetHostname() string`
- `GetSerialnumber() string`
- `GetModel() string`
- `GetWorkingDirectory() string`
- `Sleep()`
- `Shutdown()`
- `DisplaySleep()`
- `DisplayWake()`
- `Screensaver()`
- `KeepAwake()`
- `AllowSleep()`
- `GetCaffeinateStatus() bool`
- `RunShortcut(name string)`

**Aktueller Code:** Zeilen 250-516 in mac2mqtt.go

### macos/audio.go
**Verantwortlichkeit:** Audio-Steuerung

**Funktionen:**
- `GetVolume() int`
- `SetVolume(volume int) error`
- `GetMuteStatus() bool`
- `SetMute(muted bool) error`

**Aktueller Code:** Zeilen 327-465 in mac2mqtt.go

### macos/display.go
**Verantwortlichkeit:** Display-Steuerung mit BetterDisplay CLI

**Funktionen:**
- `type Display struct` - Display-Informationen
- `GetDisplays() ([]Display, error)`
- `IsDisplayAvailable(displayID string) bool`
- `GetDisplayBrightness(displayID string) (int, error)`
- `SetDisplayBrightness(displayID string, brightness int) error`
- `IsBetterDisplayCLIAvailable() bool`

**Aktueller Code:** Zeilen 517-607 in mac2mqtt.go

### macos/media.go
**Verantwortlichkeit:** Media-Kontrolle

**Funktionen:**
- `type MediaInfo struct` - Media-Informationen
- `GetMediaInfo() (*MediaInfo, error)`
- `PlayPause() error`
- `IsMediaControlAvailable() bool`

**Aktueller Code:** Zeilen 608-725 in mac2mqtt.go

### macos/monitoring.go
**Verantwortlichkeit:** System-Monitoring

**Funktionen:**
- `type DiskUsage struct`
- `type CPUUsage struct`
- `type MemoryUsage struct`
- `type UptimeInfo struct`
- `GetDiskUsage() (*DiskUsage, error)`
- `GetCPUUsage() (*CPUUsage, error)`
- `GetMemoryUsage() (*MemoryUsage, error)`
- `GetSystemUptime() (*UptimeInfo, error)`
- `GetBatteryChargePercent() string`
- `GetSystemIdleTime() (int, error)`
- `GetMediaDevicesState() (isMicOn bool, isCameraOn bool, err error)`
- `GetPublicIP() (string, error)`

**Aktueller Code:** Zeilen 1463-1777 in mac2mqtt.go

### mqtt/client.go
**Verantwortlichkeit:** MQTT-Client-Management

**Funktionen:**
- `type Client struct` - MQTT-Client-Wrapper
- `NewClient(config *config.Config) (*Client, error)`
- `Connect() error`
- `Disconnect()`
- `IsConnected() bool`
- `Publish(topic string, payload interface{}) error`
- `Subscribe(topic string, handler MessageHandler) error`

**Aktueller Code:** Zeilen 1165-1262 in mac2mqtt.go

### mqtt/handlers.go
**Verantwortlichkeit:** MQTT-Message-Handler

**Funktionen:**
- `HandleVolumeCommand(payload string) error`
- `HandleMuteCommand(payload string) error`
- `HandleSystemCommand(payload string) error`
- `HandleDisplayBrightnessCommand(displayID string, payload string) error`
- `HandleShortcutCommand(payload string) error`
- `HandleKeepAwakeCommand(payload string) error`
- `HandlePlayPauseCommand(payload string) error`

**Aktueller Code:** Zeilen 1261-1445 in mac2mqtt.go

### mqtt/discovery.go
**Verantwortlichkeit:** Home Assistant MQTT Discovery

**Funktionen:**
- `PublishDiscoveryConfig(client *Client, hostname string) error`
- `createDeviceConfig() map[string]interface{}`
- `createSensorConfig(name string, topic string) map[string]interface{}`
- ... weitere Discovery-Hilfsfunktionen

**Aktueller Code:** Zeilen 1778-2300 in mac2mqtt.go

## Umsetzungsschritte

### Phase 1: Config-Package (abgeschlossen ✅)
- [x] Verzeichnis config/ erstellt
- [x] config/config.go mit Config-Struktur erstellt
- [x] Config-Logik vollständig ausgelagert (LoadConfig, Validate)
- [x] Error handling verbessert

### Phase 2: macOS-Package (abgeschlossen ✅)
- [x] Verzeichnis macos/ erstellt
- [x] macos/commands.go mit System-Kommandos (Sleep, Shutdown, etc.)
- [x] macos/audio.go mit Audio-Steuerung (Volume, Mute)
- [x] macos/display.go mit Display-Kontrolle (Brightness)
- [x] macos/media.go mit Media-Kontrolle (Play/Pause, MediaInfo)
- [x] macos/monitoring.go mit System-Monitoring (CPU, RAM, Disk, Battery, etc.)
- [x] Tests für macOS-Funktionen laufen erfolgreich

### Phase 3: MQTT-Package erstellen (übersprungen ⏭️)
- [ ] mqtt/client.go erstellen
- [ ] mqtt/handlers.go erstellen
- [ ] mqtt/discovery.go erstellen
- [ ] Tests für MQTT-Funktionen

**Hinweis:** Phase 3 wurde übersprungen, da dies eine umfangreiche Umstrukturierung der Application-Struktur erfordern würde. Kann als zukünftige Verbesserung implementiert werden.

### Phase 4: Hauptanwendung anpassen (abgeschlossen ✅)
- [x] Alle Funktionsaufrufe auf neue Packages umgestellt
- [x] Duplizierte Funktionen aus mac2mqtt.go entfernt
- [x] Type-Aliase für Rückwärtskompatibilität erstellt
- [x] Ungenutzte Imports bereinigt
- [x] Tests auf neue Package-Struktur aktualisiert

**Ergebnis:**
- mac2mqtt.go: 2410 → 1743 Zeilen (**-667 Zeilen, -28%**)
- Build erfolgreich: ✅
- Alle Tests bestehen: ✅ (11/11 Tests PASS)

### Phase 5: Testing & Dokumentation (abgeschlossen ✅)
- [x] Basis-Tests angepasst und erweitert
- [x] go build && go test erfolgreich nach vollständiger Migration
- [x] Alle 11 Tests bestehen
- [ ] README.md mit neuer Struktur aktualisieren (optional)
- [ ] Dokumentation der API-Funktionen (optional)

## Abschluss-Status

### Vollständig implementiert ✅
- **config/** Package (1 Datei, LoadConfig + Validate)
- **macos/** Package (5 Dateien):
  - commands.go (163 Zeilen) - Systemkommandos
  - audio.go (157 Zeilen) - Lautstärke & Mute
  - display.go (130 Zeilen) - Display-Helligkeit
  - media.go (134 Zeilen) - Media-Steuerung
  - monitoring.go (273 Zeilen) - System-Monitoring
- **mac2mqtt.go** refactored (1743 Zeilen, -28% Reduktion)
- **Tests** aktualisiert (11/11 PASS)

### Übersprungen ⏭️
- **mqtt/** Package - Würde größere Application-Refactoring erfordern

## Vorteile nach dem Refactoring

1. **Bessere Wartbarkeit:** Kleinere, fokussierte Dateien statt einer 2410-Zeilen-Datei
2. **Testbarkeit:** Packages können isoliert getestet werden
3. **Wiederverwendbarkeit:** Packages können in anderen Projekten genutzt werden
4. **Klarere Verantwortlichkeiten:** Jedes Package hat einen klar definierten Zweck
5. **Einfacheres Onboarding:** Neue Entwickler finden sich schneller zurecht

## Nächste Schritte

Um das Refactoring abzuschließen:

1. Führe `make refactor-config` aus (wenn Makefile vorhanden)
2. Oder: Arbeite die Phasen manuell ab
3. Nach jeder Phase: `go build && go test` ausführen
4. Bei Fehlern: Importe und Funktionsaufrufe anpassen

## Hinweise

- Alle exportierten Funktionen (für den Zugriff von außen) müssen mit Großbuchstaben beginnen
- Interne Hilfsfunktionen sollten klein geschrieben werden
- Dependencies zwischen Packages vermeiden (z.B. macos sollte nicht mqtt importieren)
- Fehlerbehandlung konsistent gestalten
