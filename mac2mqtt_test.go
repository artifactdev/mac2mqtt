package main

import (
	"bessarabov/mac2mqtt/macos"
	"os/exec"
	"testing"
)

func TestMac2MQTTBinaryExists(t *testing.T) {
    if _, err := exec.LookPath("./mac2mqtt"); err != nil {
        t.Fatalf("mac2mqtt Binary nicht gefunden im aktuellen Verzeichnis: %v", err)
    }
}

func TestMac2MQTTStart(t *testing.T) {
    cmd := exec.Command("./mac2mqtt")
    if err := cmd.Start(); err != nil {
        t.Fatalf("mac2mqtt konnte nicht gestartet werden: %v", err)
    }
    cmd.Process.Kill() // Sofort beenden, da wir nur den Start testen
}

func TestGetHostname(t *testing.T) {
    host := macos.GetHostname()
    if host == "" {
        t.Error("Hostname sollte nicht leer sein")
    }
}

func TestGetModel(t *testing.T) {
    model := macos.GetModel()
    if model == "" {
        t.Error("Model sollte nicht leer sein")
    }
}

func TestGetSerialnumber(t *testing.T) {
    serial := macos.GetSerialnumber()
    if serial == "" {
        t.Error("Serialnumber sollte nicht leer sein")
    }
}

func TestGetWorkingDirectory(t *testing.T) {
    wd := macos.GetWorkingDirectory()
    if wd == "" {
        t.Error("WorkingDirectory sollte nicht leer sein")
    }
}

func TestGetMuteStatus(t *testing.T) {
    _ = macos.GetMuteStatus() // Kann true/false sein, Test auf Fehlerfreiheit
}

func TestGetCurrentVolume(t *testing.T) {
    vol := macos.GetVolume()
    if vol < 0 || vol > 100 {
        t.Errorf("Volume außerhalb des Bereichs: %d", vol)
    }
}

func TestGetDiskUsage(t *testing.T) {
    disk, err := macos.GetDiskUsage()
    if err != nil {
        t.Errorf("Fehler bei getDiskUsage: %v", err)
    }
    if disk != nil && disk.Total == 0 {
        t.Error("Disk Total sollte > 0 sein")
    }
}

func TestGetMemoryUsage(t *testing.T) {
    mem, err := macos.GetMemoryUsage()
    if err != nil {
        t.Errorf("Fehler bei getMemoryUsage: %v", err)
    }
    if mem != nil && mem.Total == 0 {
        t.Error("Memory Total sollte > 0 sein")
    }
}

func TestGetSystemUptime(t *testing.T) {
    uptime, err := macos.GetSystemUptime()
    if err != nil {
        t.Errorf("Fehler bei getSystemUptime: %v", err)
    }
    if uptime != nil && uptime.Seconds == 0 {
        t.Error("Uptime sollte > 0 sein")
    }
}
