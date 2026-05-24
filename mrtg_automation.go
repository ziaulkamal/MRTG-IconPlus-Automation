package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
}

// MRTGAutomator menyimpan konfigurasi sesi Chrome
type MRTGAutomator struct {
	timeout time.Duration
}

func NewMRTGAutomator() *MRTGAutomator {
	return &MRTGAutomator{timeout: 60 * time.Minute}
}

// ChromeContext membungkus lifecycle Chrome
type ChromeContext struct {
	Ctx    context.Context
	Cancel context.CancelFunc
}

func (cc *ChromeContext) Done() { cc.Cancel() }

func (m *MRTGAutomator) setupChrome() *ChromeContext {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, m.timeout)

	return &ChromeContext{Ctx: timeoutCtx, Cancel: func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}}
}

func (m *MRTGAutomator) login(ctx context.Context, cfg *CaptureConfig) error {
	username, password := cfg.Username, cfg.Password
	if cfg.UseSID {
		username = cfg.LoginSID
		password = cfg.LoginSID
	}
	return chromedp.Run(ctx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="username"]`, username, chromedp.ByQuery),
		chromedp.SetValue(`input[name="password"]`, password, chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`body`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	)
}

// captureRange mengambil chart MRTG untuk rentang waktu startDT–endDT.
// outputFile adalah path lengkap file output termasuk nama file.
func (m *MRTGAutomator) captureRange(ctx context.Context, sid string, startDT, endDT time.Time, outputFile string) error {
	startStr := startDT.Format("2006/01/02 15:04")
	endStr := endDT.Format("2006/01/02 15:04")

	if err := chromedp.Run(ctx,
		chromedp.Navigate(graphBaseURL+sid),
		chromedp.WaitVisible(`input[name="start"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
	); err != nil {
		return fmt.Errorf("navigasi gagal: %w", err)
	}

	jsSetDate := fmt.Sprintf(`
		(function() {
			var s = document.querySelector('input[name="start"]');
			var e = document.querySelector('input[name="end"]');
			if (!s || !e) return false;
			s.value = '%s'; e.value = '%s';
			['change','input'].forEach(function(ev) {
				s.dispatchEvent(new Event(ev, {bubbles:true}));
				e.dispatchEvent(new Event(ev, {bubbles:true}));
			});
			return true;
		})()
	`, startStr, endStr)

	var ok bool
	if err := chromedp.Run(ctx, chromedp.Evaluate(jsSetDate, &ok)); err != nil || !ok {
		return fmt.Errorf("set tanggal gagal")
	}

	if err := chromedp.Run(ctx,
		chromedp.Click(`button[type="submit"].btn-success`, chromedp.ByQuery),
		chromedp.WaitVisible(`div#linechart_core`, chromedp.ByQuery),
		chromedp.WaitVisible(`div#nvd3-line`, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		return fmt.Errorf("chart tidak muncul: %w", err)
	}

	var buf []byte
	if err := chromedp.Run(ctx,
		chromedp.Screenshot(`div#nvd3-line`, &buf, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("screenshot gagal: %w", err)
	}

	return os.WriteFile(outputFile, buf, 0644)
}

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

// errCancelled adalah sentinel error untuk pembatalan oleh pengguna.
var errCancelled = fmt.Errorf("dibatalkan")

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

// extractSIDTitle mengambil teks judul SID dari elemen SVG <text text-anchor="start">
// yang memuat judul chart MRTG (mis. "231202003123 - Metro P2MP Diskominfo ...").
func (m *MRTGAutomator) extractSIDTitle(ctx context.Context) string {
	var title string
	js := `(function() {
		// Metode 1 (paling andal): elemen <text text-anchor="start"> di dalam SVG
		// – ini persis elemen judul chart NVD3 yang dimuat halaman MRTG
		try {
			var els = document.querySelectorAll('text[text-anchor="start"]');
			for (var i = 0; i < els.length; i++) {
				var t = (els[i].textContent || '').trim();
				if (t.length > 8) return t;
			}
		} catch(e) {}

		// Metode 2: semua <text> di dalam <svg>, ambil yang terpanjang
		try {
			var svgTexts = document.querySelectorAll('svg text');
			var best = '';
			for (var i = 0; i < svgTexts.length; i++) {
				var s = svgTexts[i].textContent.trim();
				if (s.length > best.length) best = s;
			}
			if (best.length > 5) return best;
		} catch(e) {}

		// Metode 3: document.title sebagai fallback terakhir
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

// captureTask adalah satu unit pekerjaan yang dikerjakan worker.
type captureTask struct {
	sid     string
	startDT time.Time
	endDT   time.Time
	outFile string
	label   string
}

// runCapture menjalankan seluruh proses otomasi dengan worker pool paralel.
// userCtx → context pembatalan dari pengguna (tombol Batalkan di GUI)
// logFn   → callback untuk output log (dipanggil dari banyak goroutine, mutex internal)
// progFn  → callback progress (done, total); boleh nil
func runCapture(userCtx context.Context, cfg *CaptureConfig, logFn func(string), progFn func(done, total int)) error {
	if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
		return fmt.Errorf("gagal buat folder output: %w", err)
	}

	// ── Bangun daftar tugas ───────────────────────────────────────────────────
	var tasks []captureTask
	if cfg.IsMonthly {
		startDT := time.Date(cfg.StartDate.Year(), cfg.StartDate.Month(), 1, 0, 0, 0, 0, time.Local)
		endDT := time.Date(cfg.EndDate.Year(), cfg.EndDate.Month(), cfg.EndDate.Day(), 23, 59, 0, 0, time.Local)
		periodLabel := fmt.Sprintf("%s – %s", startDT.Format("01/2006"), endDT.Format("02/01/2006"))
		for _, sid := range cfg.SIDs {
			sid := sid
			tasks = append(tasks, captureTask{
				sid:     sid,
				startDT: startDT,
				endDT:   endDT,
				outFile: filepath.Join(cfg.OutputDir, fmt.Sprintf("%s_%s.jpg", sid, startDT.Format("01-2006"))),
				label:   fmt.Sprintf("%s (bulanan %s)", sid, periodLabel),
			})
		}
	} else {
		dates := generateDateRange(cfg.StartDate, cfg.EndDate)
		for _, sid := range cfg.SIDs {
			sid := sid
			for _, date := range dates {
				date := date
				startDT := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.Local)
				endDT := time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 0, 0, time.Local)
				tasks = append(tasks, captureTask{
					sid:     sid,
					startDT: startDT,
					endDT:   endDT,
					outFile: filepath.Join(cfg.OutputDir, fmt.Sprintf("%s_%s.jpg", sid, date.Format("02-01-2006"))),
					label:   fmt.Sprintf("%s_%s.jpg", sid, date.Format("02-01-2006")),
				})
			}
		}
	}

	total := len(tasks)

	// ── Tentukan jumlah worker ────────────────────────────────────────────────
	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}
	if workers > total {
		workers = total
	}

	logFn(fmt.Sprintf("🚀 Memulai capture: %d tugas, %d worker paralel", total, workers))

	// Distribusi tugas lewat channel buffered
	taskCh := make(chan captureTask, total)
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	// ── Counter + metadata thread-safe ───────────────────────────────────────
	var mu sync.Mutex
	done, failed := 0, 0
	sidMeta := make(map[string]string) // SID → tipe (Backhaul/METRONET/INTERNET)

	safeLog := func(msg string) {
		mu.Lock()
		logFn(msg)
		mu.Unlock()
	}
	addDone := func(fail bool) (d, t int) {
		mu.Lock()
		defer mu.Unlock()
		if fail {
			failed++
		}
		done++
		return done, total
	}

	// ── Spawn worker goroutines ───────────────────────────────────────────────
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			automator := NewMRTGAutomator()
			chrome := automator.setupChrome()
			defer chrome.Done()

			// Batalkan Chrome saat userCtx dibatalkan
			go func() {
				select {
				case <-userCtx.Done():
					chrome.Done()
				case <-chrome.Ctx.Done():
				}
			}()

			if err := automator.login(chrome.Ctx, cfg); err != nil {
				safeLog(fmt.Sprintf("❌ Worker %d: login gagal: %v", workerID+1, err))
				// Kosongkan tugas milik worker ini agar tidak diproses
				for range taskCh {
				}
				return
			}
			safeLog(fmt.Sprintf("✅ Worker %d: login berhasil", workerID+1))

			for task := range taskCh {
				if userCtx.Err() != nil {
					return
				}

				if err := automator.captureRange(chrome.Ctx, task.sid, task.startDT, task.endDT, task.outFile); err != nil {
					safeLog(fmt.Sprintf("  ❌ %s: %v", task.label, err))
					d, t := addDone(true)
					if progFn != nil {
						progFn(d, t)
					}
				} else {
					// Ekstrak tipe SID dari halaman (hanya sekali per SID)
					mu.Lock()
					_, alreadyTyped := sidMeta[task.sid]
					mu.Unlock()
					if !alreadyTyped {
						rawTitle := automator.extractSIDTitle(chrome.Ctx)
						sidType := detectSIDType(rawTitle)
						mu.Lock()
						sidMeta[task.sid] = sidType
						mu.Unlock()
						safeLog(fmt.Sprintf("  ✅ %s tersimpan [%s]", task.label, sidType))
					} else {
						safeLog(fmt.Sprintf("  ✅ %s tersimpan", task.label))
					}
					d, t := addDone(false)
					if progFn != nil {
						progFn(d, t)
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Simpan metadata tipe SID ke capture_meta.json
	if len(sidMeta) > 0 {
		saveSIDMeta(cfg.OutputDir, sidMeta)
		logFn(fmt.Sprintf("💾 Metadata tipe SID disimpan (%d entri)", len(sidMeta)))
	}

	if userCtx.Err() != nil {
		logFn("⛔ Capture dibatalkan oleh pengguna")
		return errCancelled
	}

	logFn("\n══════════ Selesai ══════════")
	logFn(fmt.Sprintf("✅ Berhasil : %d", done-failed))
	if failed > 0 {
		logFn(fmt.Sprintf("❌ Gagal    : %d", failed))
	}
	return nil
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
	// Jika tidak ada argumen → buka GUI
	// Jika ada argumen apapun (--login, --user, dsb) → CLI
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
