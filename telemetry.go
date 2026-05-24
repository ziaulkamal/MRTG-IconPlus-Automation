package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ── Types ────────────────────────────────────────────────────────────────────

type heartbeatPayload struct {
	MAC     string `json:"mac"`
	Version string `json:"version"`
}

type heartbeatResponse struct {
	Blocked bool `json:"blocked"`
}

type countResponse struct {
	Active int `json:"active"`
}

// ── State ────────────────────────────────────────────────────────────────────

var activeUserCount int64 // updated by background poller

// GetActiveCount returns the last-known active user count from the server.
func GetActiveCount() int {
	return int(atomic.LoadInt64(&activeUserCount))
}

// ── MAC address ──────────────────────────────────────────────────────────────

// getMACAddress returns the first non-loopback hardware MAC on the system.
func getMACAddress() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		mac := iface.HardwareAddr.String()
		if mac != "" && mac != "<nil>" {
			return mac
		}
	}
	return "unknown"
}

// ── HTTP helpers ─────────────────────────────────────────────────────────────

var telemetryClient = &http.Client{Timeout: 8 * time.Second}

func postJSON(url string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := telemetryClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if target == nil {
		return nil
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

func getJSON(url string, target any) error {
	resp, err := telemetryClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}

// ── Telemetry loop ───────────────────────────────────────────────────────────

// startTelemetry begins heartbeat pings to the tracking server and polls
// active count. Shows a block dialog if the server marks this client blocked.
func startTelemetry(w fyne.Window) {
	mac := getMACAddress()

	sendBeat := func() bool {
		var resp heartbeatResponse
		err := postJSON(
			trackingServerURL+"/api/heartbeat",
			heartbeatPayload{MAC: mac, Version: appVersion},
			&resp,
		)
		if err != nil {
			return false // server offline — don't block
		}
		return resp.Blocked
	}

	refreshCount := func() {
		var cr countResponse
		if err := getJSON(trackingServerURL+"/api/count", &cr); err == nil {
			atomic.StoreInt64(&activeUserCount, int64(cr.Active))
		}
	}

	go func() {
		blocked := sendBeat()
		refreshCount()
		if blocked {
			fyne.Do(func() { showBlockedDialog(w) })
			return
		}

		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			blocked := sendBeat()
			refreshCount()
			if blocked {
				fyne.Do(func() { showBlockedDialog(w) })
				return
			}
		}
	}()
}

func showBlockedDialog(w fyne.Window) {
	msg := fmt.Sprintf(
		"Perangkat ini telah diblokir oleh administrator.\n"+
			"Hubungi tim PLN Icon Plus untuk informasi lebih lanjut.\n\n"+
			"Versi: %s", appVersion)
	dialog.ShowCustom(
		"Akses Diblokir",
		"Keluar",
		widget.NewLabel(msg),
		w,
	)
}
