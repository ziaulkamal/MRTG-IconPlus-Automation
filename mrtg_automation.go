package main

// mrtg_automation.go — tipe data, helper, CLI, dan main.
// Semua logika browser automation ada di mrtg_engine.go.

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

const (
	loginURL     = "https://mrtg.iconpln.co.id/login"
	graphBaseURL = "https://mrtg.iconpln.co.id/graph/custom/"
)

// CaptureConfig menyimpan semua parameter yang dibutuhkan
type CaptureConfig struct {
	UseSID    bool
	LoginSID  string
	Username  string
	Password  string
	SIDs      []string
	StartDate time.Time
	EndDate   time.Time
	OutputDir string
	// IsMonthly = true  → laporan bulanan: satu chart per SID mencakup seluruh periode
	// IsMonthly = false → laporan harian: satu chart per hari per SID
	IsMonthly bool
	// Workers = jumlah Chrome paralel; 0 atau 1 = sekuensial
	Workers int
	// DNS = URL DNS-over-HTTPS; kosong = pakai DNS sistem/ISP
	// Contoh: "https://dns.google/dns-query" untuk Google DNS
	DNS string
}

// MRTGAutomator menyimpan konfigurasi sesi Chrome
type MRTGAutomator struct {
	timeout time.Duration
}

// ChromeContext membungkus lifecycle Chrome
type ChromeContext struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

func (cc *ChromeContext) Done() { cc.Cancel() }

// captureTask adalah satu unit pekerjaan yang dikerjakan worker.
type captureTask struct {
	sid     string
	startDT time.Time
	endDT   time.Time
	outFile string
	label   string
}

// errCancelled adalah sentinel error untuk pembatalan oleh pengguna.
var errCancelled = fmt.Errorf("dibatalkan")

// ─── Helper functions ─────────────────────────────────────────────────────────

// generateDateRange menghasilkan slice tanggal dari start s/d end (inclusive)
func generateDateRange(start, end time.Time) []time.Time {
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.Local)
	end = time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.Local)
	var dates []time.Time
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d)
	}
	return dates
}

// detectSIDType menentukan tipe layanan dari teks judul chart MRTG.
// Prioritas: Backhaul > METRONET > INTERNET
func detectSIDType(title string) string {
	lower := strings.ToLower(title)
	if strings.Contains(lower, "backhaul") {
		return "Backhaul"
	}
	if strings.Contains(lower, "metro") {
		return "METRONET"
	}
	return "INTERNET"
}

// extractSIDTitle mengambil teks judul SID dari elemen SVG chart MRTG.
func (m *MRTGAutomator) extractSIDTitle(ctx context.Context) string {
	var title string
	js := `(function() {
		try {
			var els = document.querySelectorAll('text[text-anchor="start"]');
			for (var i = 0; i < els.length; i++) {
				var t = (els[i].textContent || '').trim();
				if (t.length > 8) return t;
			}
		} catch(e) {}
		try {
			var svgTexts = document.querySelectorAll('svg text');
			var best = '';
			for (var i = 0; i < svgTexts.length; i++) {
				var s = svgTexts[i].textContent.trim();
				if (s.length > best.length) best = s;
			}
			if (best.length > 5) return best;
		} catch(e) {}
		return document.title || '';
	})()`
	chromedp.Run(ctx, chromedp.Evaluate(js, &title)) //nolint:errcheck
	return title
}

// saveSIDMeta menyimpan peta SID→tipe ke file capture_meta.json di outputDir.
func saveSIDMeta(outputDir string, meta map[string]string) {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(filepath.Join(outputDir, "capture_meta.json"), data, 0644) //nolint:errcheck
}

// readSIDsFromFile membaca daftar SID dari file (satu per baris, # untuk komentar)
func readSIDsFromFile(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var sids []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			sids = append(sids, line)
		}
	}
	return sids, sc.Err()
}

func ask(sc *bufio.Scanner, label string) string {
	fmt.Print(label)
	sc.Scan()
	return strings.TrimSpace(sc.Text())
}

// ─── CLI mode ─────────────────────────────────────────────────────────────────

// runCLI menjalankan mode terminal interaktif
func runCLI() {
	sc := bufio.NewScanner(os.Stdin)
	cfg := &CaptureConfig{}

	fmt.Println("╔══════════════════════════════════╗")
	fmt.Println("║    MRTG Chart Automation Tool    ║")
	fmt.Println("╚══════════════════════════════════╝")

	// ── Metode Login ──────────────────────────────────────────────────────────
	fmt.Println("\nMetode login:")
	fmt.Println("  [1] Via SID  (SID digunakan sebagai username & password)")
	fmt.Println("  [2] Via Username & Password")
	switch ask(sc, "Pilihan (1/2): ") {
	case "1":
		cfg.UseSID = true
		cfg.LoginSID = ask(sc, "Masukan SID login: ")
		if cfg.LoginSID == "" {
			log.Fatal("❌ SID tidak boleh kosong")
		}
	case "2":
		cfg.UseSID = false
		cfg.Username = ask(sc, "Username: ")
		cfg.Password = ask(sc, "Password: ")
		if cfg.Username == "" || cfg.Password == "" {
			log.Fatal("❌ Username dan password tidak boleh kosong")
		}
	default:
		log.Fatal("❌ Pilihan tidak valid")
	}

	// ── Mode Capture ──────────────────────────────────────────────────────────
	fmt.Println("\nMode capture:")
	fmt.Println("  [1] Single SID")
	fmt.Println("  [2] Multiple SID (dari file)")
	switch ask(sc, "Pilihan (1/2): ") {
	case "1":
		sid := ask(sc, "\nSID Layanan: ")
		if sid == "" {
			log.Fatal("❌ SID tidak boleh kosong")
		}
		cfg.SIDs = []string{sid}
	case "2":
		fmt.Println("\nFormat file: satu SID per baris, baris '#' diabaikan")
		fmt.Println("Contoh path: C:\\Users\\Asani\\Desktop\\sids.txt")
		path := ask(sc, "Path file  : ")
		sids, err := readSIDsFromFile(path)
		if err != nil {
			log.Fatalf("❌ Gagal baca file: %v", err)
		}
		if len(sids) == 0 {
			log.Fatal("❌ File SID kosong")
		}
		cfg.SIDs = sids
		fmt.Printf("✅ %d SID ditemukan\n", len(sids))
	default:
		log.Fatal("❌ Pilihan tidak valid")
	}

	// ── Rentang Tanggal ───────────────────────────────────────────────────────
	fmt.Println("\nJenis laporan:")
	fmt.Println("  [1] Laporan Bulanan  (1 file per SID, grafik mencakup 1 bulan penuh)")
	fmt.Println("  [2] Laporan Harian   (1 file per hari per SID, iterasi tiap tanggal)")
	switch ask(sc, "Pilihan (1/2): ") {
	case "1":
		val := ask(sc, "\nBulan (YYYY/MM) contoh 2026/06: ")
		t, err := time.Parse("2006/01", val)
		if err != nil {
			log.Fatal("❌ Format tidak valid. Gunakan YYYY/MM")
		}
		cfg.IsMonthly = true
		cfg.StartDate = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
		now := time.Now()
		if t.Year() == now.Year() && t.Month() == now.Month() {
			cfg.EndDate = now
		} else {
			cfg.EndDate = time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.Local)
		}
	case "2":
		cfg.IsMonthly = false
		startVal := ask(sc, "\nTanggal mulai (YYYY/MM/DD) contoh 2026/06/05: ")
		endVal := ask(sc, "Tanggal akhir (YYYY/MM/DD) contoh 2026/06/20: ")
		s, err1 := time.Parse("2006/01/02", startVal)
		e, err2 := time.Parse("2006/01/02", endVal)
		if err1 != nil || err2 != nil {
			log.Fatal("❌ Format tidak valid. Gunakan YYYY/MM/DD")
		}
		if e.Before(s) {
			log.Fatal("❌ Tanggal akhir tidak boleh sebelum tanggal mulai")
		}
		cfg.StartDate = s
		cfg.EndDate = e
	default:
		log.Fatal("❌ Pilihan tidak valid")
	}

	// ── DNS ───────────────────────────────────────────────────────────────────
	fmt.Println("\nDNS Resolver (pilih jika DNS ISP sering error):")
	for i, name := range DNSPresetNames {
		fmt.Printf("  [%d] %s\n", i, name)
	}
	dnsIdx := ask(sc, "Pilihan DNS (0=default): ")
	idx := 0
	fmt.Sscanf(dnsIdx, "%d", &idx)
	if idx >= 0 && idx < len(DNSPresetNames) {
		cfg.DNS = DNSPresets[DNSPresetNames[idx]]
	}

	// ── Output ────────────────────────────────────────────────────────────────
	fmt.Println("\nFolder penyimpanan chart:")
	fmt.Println("  Contoh Windows : C:\\Users\\Asani\\Desktop\\charts")
	fmt.Println("  Contoh relatif : ./output")
	baseDir := ask(sc, "Folder output  : ")
	if baseDir == "" {
		baseDir = "./output"
		fmt.Println("  (menggunakan default: ./output)")
	}
	modeLabelCLI := "Harian"
	if cfg.IsMonthly {
		modeLabelCLI = "Bulanan"
	}
	cfg.OutputDir = filepath.Join(baseDir, fmt.Sprintf("capture_%s_%s", time.Now().Format("02_01_2006"), modeLabelCLI))
	fmt.Printf("  (subfolder: %s)\n", cfg.OutputDir)

	// ── Jumlah Worker ────────────────────────────────────────────────────────
	fmt.Println("\nJumlah worker paralel (Chrome berjalan bersamaan):")
	fmt.Println("  Rekomendasi: 3 untuk koneksi stabil, 5 untuk koneksi cepat")
	wStr := ask(sc, "Worker (1-5, default 3): ")
	cfg.Workers = 3
	switch wStr {
	case "1":
		cfg.Workers = 1
	case "2":
		cfg.Workers = 2
	case "4":
		cfg.Workers = 4
	case "5":
		cfg.Workers = 5
	}

	// ── Ringkasan ─────────────────────────────────────────────────────────────
	fmt.Println("\n──────── Konfigurasi ────────")
	if cfg.UseSID {
		fmt.Printf("Login       : SID %s\n", cfg.LoginSID)
	} else {
		fmt.Printf("Login       : %s\n", cfg.Username)
	}
	fmt.Printf("Jumlah SID  : %d\n", len(cfg.SIDs))
	if len(cfg.SIDs) <= 10 {
		for i, s := range cfg.SIDs {
			fmt.Printf("  [%d] %s\n", i+1, s)
		}
	}
	fmt.Printf("Periode     : %s s/d %s\n",
		cfg.StartDate.Format("02-01-2006"), cfg.EndDate.Format("02-01-2006"))
	if cfg.IsMonthly {
		fmt.Printf("Jenis       : Laporan Bulanan\n")
		fmt.Printf("Total file  : %d  (1 per SID)\n", len(cfg.SIDs))
		fmt.Printf("Format file : [SID]_[MM-YYYY].jpg\n")
	} else {
		dates := generateDateRange(cfg.StartDate, cfg.EndDate)
		fmt.Printf("Jenis       : Laporan Harian\n")
		fmt.Printf("Total file  : %d  (%d SID × %d hari)\n", len(cfg.SIDs)*len(dates), len(cfg.SIDs), len(dates))
		fmt.Printf("Format file : [SID]_[DD-MM-YYYY].jpg\n")
	}
	dnsDisplay := "Sistem/ISP (default)"
	for _, name := range DNSPresetNames {
		if DNSPresets[name] == cfg.DNS && cfg.DNS != "" {
			dnsDisplay = name
			break
		}
	}
	fmt.Printf("DNS         : %s\n", dnsDisplay)
	fmt.Printf("Output dir  : %s\n", cfg.OutputDir)
	fmt.Printf("Worker      : %d (paralel)\n", cfg.Workers)
	fmt.Println("────────────────────────────")

	if strings.ToLower(ask(sc, "\nLanjutkan? (y/n): ")) != "y" {
		fmt.Println("Dibatalkan.")
		os.Exit(0)
	}

	if err := runCapture(context.Background(), cfg, func(msg string) { log.Println(msg) }, func(done, total int) {
		fmt.Printf("\r  Progress: %d/%d", done, total)
	}); err != nil {
		log.Fatalf("❌ %v", err)
	}
	fmt.Println()
}

func main() {
	if len(os.Args) == 1 {
		launchGUI()
		return
	}
	for _, arg := range os.Args[1:] {
		if arg == "--gui" || arg == "-gui" {
			launchGUI()
			return
		}
	}
	runCLI()
}
