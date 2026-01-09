package tests

package main

import (
    "os/exec"
    "testing"
)

func TestMac2MQTTBinaryExists(t *testing.T) {
    if _, err := exec.LookPath("../mac2mqtt"); err != nil {
        t.Fatalf("mac2mqtt Binary nicht gefunden im übergeordneten Verzeichnis: %v", err)
    }
}

func TestMac2MQTTStart(t *testing.T) {
    cmd := exec.Command("../mac2mqtt")
    if err := cmd.Start(); err != nil {
        t.Fatalf("mac2mqtt konnte nicht gestartet werden: %v", err)
    }
    cmd.Process.Kill() // Sofort beenden, da wir nur den Start testen
}

func TestGetHostname(t *testing.T) {
    host := getHostname()
    if host == "" {
        t.Error("Hostname sollte nicht leer sein")
    }
}

func TestGetModel(t *testing.T) {
    model := getModel()
    if model == "" {
        t.Error("Model sollte nicht leer sein")
    }
}

func TestGetSerialnumber(t *testing.T) {
    serial := getSerialnumber()
    if serial == "" {
        t.Error("Serialnumber sollte nicht leer sein")
    }
}

func TestGetWorkingDirectory(t *testing.T) {
    wd := getWorkingDirectory()
    if wd == "" {
        t.Error("WorkingDirectory sollte nicht leer sein")
    }
}

func TestGetMuteStatus(t *testing.T) {
    _ = getMuteStatus() // Kann true/false sein, Test auf Fehlerfreiheit
}

func TestGetCurrentVolume(t *testing.T) {
    vol := getCurrentVolume()
    if vol < 0 || vol > 100 {
        t.Errorf("Volume außerhalb des Bereichs: %d", vol)
    }
}

func TestGetDiskUsage(t *testing.T) {
    disk, err := getDiskUsage()
    if err != nil {
        t.Errorf("Fehler bei getDiskUsage: %v", err)
    }
    if disk != nil && disk.Total == 0 {
        t.Error("Disk Total sollte > 0 sein")
    }
}

func TestGetMemoryUsage(t *testing.T) {
    mem, err := getMemoryUsage()
    if err != nil {
        t.Errorf("Fehler bei getMemoryUsage: %v", err)
    }
    if mem != nil && mem.Total == 0 {
        t.Error("Memory Total sollte > 0 sein")
    }
}

func TestGetSystemUptime(t *testing.T) {
    uptime, err := getSystemUptime()
    if err != nil {
        t.Errorf("Fehler bei getSystemUptime: %v", err)
    }
    if uptime != nil && uptime.Seconds == 0 {
        t.Error("Uptime sollte > 0 sein")
    }
}
