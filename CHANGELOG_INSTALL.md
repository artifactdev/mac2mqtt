# Installation Script & Documentation Updates

## Änderungen am Installationsskript (install.sh)

### Pfad-Handling
- ✅ Verwendet jetzt `$HOME` statt hardcoded `/Users/username`
- ✅ Unterstützt nicht-standardmäßige Home-Verzeichnisse (z.B. `/Volumes/HubDrive/Users/...`)
- ✅ Dynamische PATH-Erstellung mit allen benötigten Tools

### LM Studio Integration
- ✅ Prüft auf LM Studio CLI Installation (`~/.lmstudio/bin/lms`)
- ✅ Zeigt Installationsanleitung wenn nicht vorhanden
- ✅ Fügt LM Studio CLI automatisch zum PATH hinzu

### LaunchAgent Setup
- ✅ Erstellt plist dynamisch mit korrekten Pfaden
- ✅ Installiert in `/Library/LaunchAgents/` (system-weit)
- ✅ Verwendet `launchctl bootstrap` statt `load`
- ✅ Inkludiert folgende Pfade in PATH:
  - `$HOME/.lmstudio/bin` (LM Studio CLI)
  - Node.js bin-Verzeichnis (dynamisch erkannt)
  - `/usr/local/bin`, `/opt/homebrew/bin`, etc.

### Management Scripts
- ✅ Verwendet `sudo launchctl bootout/bootstrap` für System-LaunchAgent
- ✅ Korrekte Pfade für Installation/Deinstallation

## Änderungen an der Dokumentation (INSTALL.md)

### Neue Inhalte
1. **LM Studio Sektion**
   - Installation und Setup-Anleitung
   - CLI-Tools Konfiguration
   - Integration mit mac2mqtt

2. **Erweiterte Fehlersuche**
   - PATH-Probleme
   - LM Studio Debugging
   - Nicht-standardmäßige Home-Verzeichnisse
   - Service Management Kommandos

3. **Advanced Configuration**
   - Umgebungsvariablen im LaunchAgent
   - Detaillierte Service-Management-Befehle
   - Manuelle plist-Anpassung

4. **Verbesserte manuelle Installation**
   - Vollständige plist-Erstellung
   - Korrekte PATH-Konfiguration
   - Dynamische Pfad-Expansion

## Vorteile der Änderungen

1. **Kompatibilität**: Funktioniert mit allen macOS-Setups, auch mit externen Laufwerken
2. **Vollständigkeit**: Alle Dependencies (BetterDisplay, Media Control, LM Studio) werden geprüft
3. **Automatisierung**: LaunchAgent wird korrekt mit allen Pfaden konfiguriert
4. **Wartbarkeit**: Bessere Fehlersuche durch detaillierte Dokumentation
5. **Offline-Modus**: App funktioniert auch wenn MQTT Broker nicht erreichbar ist

## Testing

Das Installationsskript sollte getestet werden mit:
```bash
# Aus dem Projektverzeichnis
./install.sh
```

Nach der Installation:
```bash
# Service-Status prüfen
launchctl list | grep mac2mqtt

# Logs ansehen
tail -f /tmp/mac2mqtt.job.err

# Testen ob LM Studio erkannt wird
grep "LM Studio" /tmp/mac2mqtt.job.err
```

## Nächste Schritte

Optional könnten noch hinzugefügt werden:
- Automatische Node.js Installation wenn npm-Pakete benötigt werden
- Brewfile für alle Dependencies
- GitHub Actions für automatische Releases
