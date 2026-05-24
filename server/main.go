// Tracking server for MRTG Chart Automation.
// Deploy to a VPS and run: MASTER_KEY=yourpassword go run .
// Exposes REST API consumed by the desktop client.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// ── Config ────────────────────────────────────────────────────────────────────

const (
	port            = ":24001"
	dataFile        = "tracker_data.json"
	inactiveTimeout = 45 * time.Second // client considered offline after this
)

func masterKey() string {
	if v := os.Getenv("MASTER_KEY"); v != "" {
		return v
	}
	return "iconplus2026" // default — override via env in production
}

// ── Data model ────────────────────────────────────────────────────────────────

type UserRecord struct {
	MAC      string    `json:"mac"`
	IP       string    `json:"ip"`
	City     string    `json:"city"`
	Country  string    `json:"country"`
	ISP      string    `json:"isp"`
	Lat      float64   `json:"lat"`
	Lon      float64   `json:"lon"`
	Version  string    `json:"version"`
	LastSeen time.Time `json:"last_seen"`
	Blocked  bool      `json:"blocked"`
}

type Store struct {
	mu    sync.RWMutex
	Users map[string]*UserRecord `json:"users"` // keyed by MAC
}

var store = &Store{Users: make(map[string]*UserRecord)}

// ── Persistence ───────────────────────────────────────────────────────────────

func loadStore() {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	json.Unmarshal(data, store) //nolint:errcheck
}

func saveStore() {
	store.mu.RLock()
	data, _ := json.MarshalIndent(store, "", "  ")
	store.mu.RUnlock()
	os.WriteFile(dataFile, data, 0644) //nolint:errcheck
}

// ── IP Geolocation ────────────────────────────────────────────────────────────

type geoResult struct {
	City    string  `json:"city"`
	Country string  `json:"country"`
	ISP     string  `json:"isp"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	Status  string  `json:"status"`
}

func geolocate(ip string) geoResult {
	url := "http://ip-api.com/json/" + ip + "?fields=status,city,country,isp,lat,lon"
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return geoResult{}
	}
	defer resp.Body.Close()
	var gr geoResult
	data, _ := io.ReadAll(resp.Body)
	json.Unmarshal(data, &gr) //nolint:errcheck
	return gr
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func readJSON(r *http.Request, v any) error {
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return xff
	}
	return r.RemoteAddr
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// POST /api/heartbeat — client ping
func handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	var payload struct {
		MAC     string `json:"mac"`
		Version string `json:"version"`
	}
	if err := readJSON(r, &payload); err != nil || payload.MAC == "" {
		http.Error(w, "bad request", 400)
		return
	}

	ip := clientIP(r)

	store.mu.Lock()
	rec, exists := store.Users[payload.MAC]
	if !exists {
		rec = &UserRecord{MAC: payload.MAC}
		store.Users[payload.MAC] = rec
		store.mu.Unlock()

		// Geolocate asynchronously on first registration.
		go func() {
			geo := geolocate(ip)
			store.mu.Lock()
			rec.IP = ip
			rec.City = geo.City
			rec.Country = geo.Country
			rec.ISP = geo.ISP
			rec.Lat = geo.Lat
			rec.Lon = geo.Lon
			rec.Version = payload.Version
			rec.LastSeen = time.Now()
			store.mu.Unlock()
			saveStore()
		}()

		store.mu.RLock()
		blocked := rec.Blocked
		store.mu.RUnlock()
		writeJSON(w, map[string]bool{"blocked": blocked})
		return
	}

	rec.IP = ip
	rec.Version = payload.Version
	rec.LastSeen = time.Now()
	blocked := rec.Blocked
	store.mu.Unlock()

	saveStore()
	writeJSON(w, map[string]bool{"blocked": blocked})
}

// GET /api/count — public: returns number of clients active within timeout
func handleCount(w http.ResponseWriter, r *http.Request) {
	store.mu.RLock()
	defer store.mu.RUnlock()
	cutoff := time.Now().Add(-inactiveTimeout)
	active := 0
	for _, u := range store.Users {
		if !u.Blocked && u.LastSeen.After(cutoff) {
			active++
		}
	}
	writeJSON(w, map[string]int{"active": active})
}

// GET /api/users?key=... — master: full user list
func handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("key") != masterKey() {
		http.Error(w, "forbidden", 403)
		return
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	list := make([]*UserRecord, 0, len(store.Users))
	for _, u := range store.Users {
		list = append(list, u)
	}
	writeJSON(w, list)
}

// POST /api/block — master: block a MAC
func handleBlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key string `json:"key"`
		MAC string `json:"mac"`
	}
	if err := readJSON(r, &req); err != nil || req.Key != masterKey() {
		http.Error(w, "forbidden", 403)
		return
	}
	store.mu.Lock()
	if rec, ok := store.Users[req.MAC]; ok {
		rec.Blocked = true
	}
	store.mu.Unlock()
	saveStore()
	writeJSON(w, map[string]bool{"ok": true})
}

// POST /api/unblock — master: unblock a MAC
func handleUnblock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}
	var req struct {
		Key string `json:"key"`
		MAC string `json:"mac"`
	}
	if err := readJSON(r, &req); err != nil || req.Key != masterKey() {
		http.Error(w, "forbidden", 403)
		return
	}
	store.mu.Lock()
	if rec, ok := store.Users[req.MAC]; ok {
		rec.Blocked = false
	}
	store.mu.Unlock()
	saveStore()
	writeJSON(w, map[string]bool{"ok": true})
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	loadStore()
	log.Printf("Tracking server starting on %s", port)
	log.Printf("Master key loaded (env MASTER_KEY or default)")

	mux := http.NewServeMux()
	mux.HandleFunc("/api/heartbeat", handleHeartbeat)
	mux.HandleFunc("/api/count", handleCount)
	mux.HandleFunc("/api/users", handleUsers)
	mux.HandleFunc("/api/block", handleBlock)
	mux.HandleFunc("/api/unblock", handleUnblock)

	// Simple status page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		store.mu.RLock()
		total := len(store.Users)
		store.mu.RUnlock()
		fmt.Fprintf(w, "MRTG Tracking Server OK — %d device(s) registered\n", total)
	})

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatal(err)
	}
}
