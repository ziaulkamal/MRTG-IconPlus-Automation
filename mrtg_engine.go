package main

// mrtg_engine.go — logika browser automation yang telah dioptimalkan.
// File ini TIDAK boleh diubah oleh linter; semua perbaikan ada di sini.
// mrtg_automation.go hanya berisi tipe data, helper, CLI, dan main.

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// ─── Konstanta engine ────────────────────────────────────────────────────────

const (
	maxLoginRetry   = 3
	maxCaptureRetry = 3
	retryBaseDelay  = 3 * time.Second

	stepLoginTimeout = 45 * time.Second
	stepNavTimeout   = 35 * time.Second
	stepChartTimeout = 50 * time.Second
	pollInterval     = 600 * time.Millisecond
)

// DNSPresets memetakan nama tampilan ke URL DNS-over-HTTPS.
// Kosong ("") berarti pakai DNS sistem/ISP bawaan.
var DNSPresets = map[string]string{
	"Default (Sistem/ISP)":  "",
	"Google (8.8.8.8)":      "https://dns.google/dns-query",
	"Cloudflare (1.1.1.1)":  "https://cloudflare-dns.com/dns-query",
	"Quad9 (9.9.9.9)":       "https://dns.quad9.net/dns-query",
}

// DNSPresetNames adalah urutan tampilan di dropdown GUI.
var DNSPresetNames = []string{
	"Default (Sistem/ISP)",
	"Google (8.8.8.8)",
	"Cloudflare (1.1.1.1)",
	"Quad9 (9.9.9.9)",
}

// ─── Helper ──────────────────────────────────────────────────────────────────

// pollJS menjalankan ekspresi JS setiap pollInterval hingga mengembalikan true
// atau ctx habis. Menggantikan chromedp.Sleep yang buta terhadap kondisi nyata.
func pollJS(expr string) chromedp.ActionFunc {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for {
			var ready bool
			if err := chromedp.Evaluate(expr, &ready).Do(ctx); err != nil {
				return err
			}
			if ready {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(pollInterval):
			}
		}
	})
}

// ─── Chrome setup ─────────────────────────────────────────────────────────────

func NewMRTGAutomator() *MRTGAutomator {
	return &MRTGAutomator{timeout: 60 * time.Minute}
}

// setupChrome membuat Chrome context headless baru.
// dohURL: URL DNS-over-HTTPS untuk mengabaikan DNS ISP yang bermasalah.
// Kosong = pakai DNS sistem.
func (m *MRTGAutomator) setupChrome(dohURL string) *ChromeContext {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.WindowSize(1920, 1080),
	)
	if dohURL != "" {
		opts = append(opts,
			chromedp.Flag("dns-over-https-templates", dohURL),
			chromedp.Flag("enable-features", "DnsOverHttps"),
		)
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, ctxCancel := chromedp.NewContext(allocCtx)
	timeoutCtx, timeoutCancel := context.WithTimeout(ctx, m.timeout)

	return &ChromeContext{Ctx: timeoutCtx, Cancel: func() {
		timeoutCancel()
		ctxCancel()
		allocCancel()
	}}
}

// ─── Login & session cookie ───────────────────────────────────────────────────

// loginAndGetCookies melakukan login dan mengambil cookie sesi dalam SATU
// chromedp.Run. Menggabungkan keduanya mencegah "context canceled" yang terjadi
// ketika login dan GetCookies memakai context yang berbeda (CDP session korup).
func (m *MRTGAutomator) loginAndGetCookies(ctx context.Context, cfg *CaptureConfig) ([]*network.Cookie, error) {
	username, password := cfg.Username, cfg.Password
	if cfg.UseSID {
		username = cfg.LoginSID
		password = cfg.LoginSID
	}

	stepCtx, cancel := context.WithTimeout(ctx, stepLoginTimeout)
	defer cancel()

	// pollJS(jsLoginDone) lebih andal dari WaitVisible("body") karena body
	// selalu ada — termasuk saat login gagal dan halaman tetap di /login.
	const jsLoginDone = `!window.location.pathname.includes('/login')`

	var cookies []*network.Cookie
	if err := chromedp.Run(stepCtx,
		chromedp.Navigate(loginURL),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="username"]`, username, chromedp.ByQuery),
		chromedp.SetValue(`input[name="password"]`, password, chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		pollJS(jsLoginDone),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	); err != nil {
		return nil, err
	}

	if len(cookies) == 0 {
		return nil, fmt.Errorf("login berhasil tapi tidak ada cookie sesi")
	}
	return cookies, nil
}

// injectCookies menyuntikkan cookie sesi ke Chrome context baru tanpa login ulang.
// Semua worker menerima cookie yang sama dari satu proses login bersama.
func injectCookies(ctx context.Context, cookies []*network.Cookie) error {
	return chromedp.Run(ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		for _, c := range cookies {
			params := network.SetCookie(c.Name, c.Value).
				WithDomain(c.Domain).
				WithPath(c.Path).
				WithSecure(c.Secure).
				WithHTTPOnly(c.HTTPOnly)
			// c.Expires adalah float64 (detik epoch); -1 = session cookie (tanpa expiry).
			if c.Expires > 0 {
				expr := cdp.TimeSinceEpoch(time.Unix(int64(c.Expires), 0))
				params = params.WithExpires(&expr)
			}
			if err := params.Do(ctx); err != nil {
				return err
			}
		}
		return nil
	}))
}

// ─── Capture ─────────────────────────────────────────────────────────────────

// captureRange mengambil screenshot chart MRTG untuk rentang waktu startDT–endDT.
// Menggunakan per-step timeout dan pollJS untuk memastikan data chart sudah
// benar-benar ter-render sebelum screenshot diambil.
func (m *MRTGAutomator) captureRange(ctx context.Context, sid string, startDT, endDT time.Time, outputFile string) error {
	startStr := startDT.Format("2006/01/02 15:04")
	endStr := endDT.Format("2006/01/02 15:04")

	// Langkah 1: Navigasi — timeout terpisah agar satu navigasi hang tidak
	// memblokir context utama selama 60 menit.
	navCtx, navCancel := context.WithTimeout(ctx, stepNavTimeout)
	defer navCancel()
	if err := chromedp.Run(navCtx,
		chromedp.Navigate(graphBaseURL+sid),
		chromedp.WaitVisible(`input[name="start"]`, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("navigasi gagal: %w", err)
	}

	// Langkah 2: Set rentang tanggal via JS
	jsSetDate := fmt.Sprintf(`(function() {
		var s = document.querySelector('input[name="start"]');
		var e = document.querySelector('input[name="end"]');
		if (!s || !e) return false;
		s.value = '%s'; e.value = '%s';
		['change','input'].forEach(function(ev) {
			s.dispatchEvent(new Event(ev, {bubbles:true}));
			e.dispatchEvent(new Event(ev, {bubbles:true}));
		});
		return true;
	})()`, startStr, endStr)

	var ok bool
	if err := chromedp.Run(navCtx, chromedp.Evaluate(jsSetDate, &ok)); err != nil || !ok {
		return fmt.Errorf("set tanggal gagal")
	}

	// Langkah 3: Submit dan tunggu chart render selesai.
	// WaitVisible(div#nvd3-line) tidak cukup — container muncul sebelum data
	// NVD3 selesai digambar. pollJS(jsChartReady) menunggu sampai SVG <path>
	// dengan data nyata (attr "d" > 20 char) benar-benar ada.
	const jsChartReady = `(function() {
		var paths = document.querySelectorAll('div#nvd3-line svg path');
		for (var i = 0; i < paths.length; i++) {
			var d = paths[i].getAttribute('d');
			if (d && d.length > 20) return true;
		}
		return false;
	})()`

	chartCtx, chartCancel := context.WithTimeout(ctx, stepChartTimeout)
	defer chartCancel()
	if err := chromedp.Run(chartCtx,
		chromedp.Click(`button[type="submit"].btn-success`, chromedp.ByQuery),
		chromedp.WaitVisible(`div#linechart_core`, chromedp.ByQuery),
		pollJS(jsChartReady),
	); err != nil {
		return fmt.Errorf("chart tidak muncul: %w", err)
	}

	// Langkah 4: Screenshot
	var buf []byte
	if err := chromedp.Run(chartCtx,
		chromedp.Screenshot(`div#nvd3-line`, &buf, chromedp.ByQuery),
	); err != nil {
		return fmt.Errorf("screenshot gagal: %w", err)
	}

	return os.WriteFile(outputFile, buf, 0644)
}

// captureRangeWithRetry mencoba captureRange hingga maxRetry kali.
// Menggunakan exponential backoff dan menghormati context cancellation.
func (m *MRTGAutomator) captureRangeWithRetry(ctx context.Context, task captureTask, maxRetry int, workerID int, logFn func(string)) error {
	var lastErr error
	for attempt := 1; attempt <= maxRetry; attempt++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := m.captureRange(ctx, task.sid, task.startDT, task.endDT, task.outFile); err == nil {
			return nil
		} else {
			lastErr = err
			logFn(fmt.Sprintf("  ⚠️  Worker %d: %s percobaan %d/%d gagal: %v", workerID, task.label, attempt, maxRetry, err))
			if attempt < maxRetry {
				wait := retryBaseDelay * time.Duration(attempt)
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(wait):
				}
			}
		}
	}
	return fmt.Errorf("gagal setelah %d percobaan: %w", maxRetry, lastErr)
}

// ─── Orchestrator ─────────────────────────────────────────────────────────────

// runCapture menjalankan seluruh proses otomasi.
//
// Arsitektur dua fase:
//   - Fase 1: 1 Chrome login → ambil cookie sesi → tutup.
//     Mencegah N worker login serentak yang membebani server.
//   - Fase 2: N worker masing-masing inject cookie → langsung kerja.
//     resultCh mengumpulkan hasil tanpa mutex per-task.
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
				outFile: fmt.Sprintf("%s/%s_%s.jpg", cfg.OutputDir, sid, startDT.Format("01-2006")),
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
					outFile: fmt.Sprintf("%s/%s_%s.jpg", cfg.OutputDir, sid, date.Format("02-01-2006")),
					label:   fmt.Sprintf("%s_%s.jpg", sid, date.Format("02-01-2006")),
				})
			}
		}
	}
	total := len(tasks)

	workers := cfg.Workers
	if workers <= 0 {
		workers = 1
	}
	if workers > total {
		workers = total
	}

	dnsLabel := "sistem/ISP"
	for name, url := range DNSPresets {
		if url == cfg.DNS && url != "" {
			dnsLabel = name
			break
		}
	}
	if cfg.DNS == "" {
		dnsLabel = "sistem/ISP"
	}
	logFn(fmt.Sprintf("🚀 Memulai capture: %d tugas, %d worker, DNS: %s", total, workers, dnsLabel))

	// ── Fase 1: Login tunggal → ambil cookie sesi ─────────────────────────────
	logFn("🔑 Login sesi bersama (1 kali untuk semua worker)...")
	loginAuto := NewMRTGAutomator()
	loginChrome := loginAuto.setupChrome(cfg.DNS)

	var sessionCookies []*network.Cookie
	var loginErr error
	for attempt := 1; attempt <= maxLoginRetry; attempt++ {
		if userCtx.Err() != nil {
			loginChrome.Done()
			return errCancelled
		}
		sessionCookies, loginErr = loginAuto.loginAndGetCookies(loginChrome.Ctx, cfg)
		if loginErr == nil {
			break
		}
		logFn(fmt.Sprintf("⚠️  Login percobaan %d/%d gagal: %v", attempt, maxLoginRetry, loginErr))
		if attempt < maxLoginRetry {
			wait := retryBaseDelay * time.Duration(attempt)
			logFn(fmt.Sprintf("    Menunggu %s sebelum retry...", wait))
			select {
			case <-userCtx.Done():
				loginChrome.Done()
				return errCancelled
			case <-time.After(wait):
			}
		}
	}
	loginChrome.Done()
	if loginErr != nil {
		return fmt.Errorf("login gagal setelah %d percobaan: %w", maxLoginRetry, loginErr)
	}
	logFn(fmt.Sprintf("✅ Login berhasil — %d cookie sesi siap untuk %d worker", len(sessionCookies), workers))

	// ── Fase 2: Worker pool ───────────────────────────────────────────────────
	taskCh := make(chan captureTask, total)
	for _, t := range tasks {
		taskCh <- t
	}
	close(taskCh)

	var mu sync.Mutex
	done, failed := 0, 0
	sidMeta := make(map[string]string)

	safeLog := func(msg string) {
		mu.Lock()
		logFn(msg)
		mu.Unlock()
	}

	type captureResult struct {
		task    captureTask
		sidType string
		err     error
	}
	resultCh := make(chan captureResult, total)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			automator := NewMRTGAutomator()
			chrome := automator.setupChrome(cfg.DNS)
			defer chrome.Done()

			// watchStop mencegah goroutine pemantau leak saat worker selesai normal.
			watchStop := make(chan struct{})
			defer close(watchStop)
			go func() {
				select {
				case <-userCtx.Done():
					chrome.Done()
				case <-chrome.Ctx.Done():
				case <-watchStop:
				}
			}()

			// Inject cookie bersama — tidak perlu login mandiri per worker.
			if err := injectCookies(chrome.Ctx, sessionCookies); err != nil {
				safeLog(fmt.Sprintf("❌ Worker %d: gagal inject cookie: %v", workerID+1, err))
				return
			}
			valCtx, valCancel := context.WithTimeout(chrome.Ctx, 20*time.Second)
			valErr := chromedp.Run(valCtx, chromedp.Navigate("https://mrtg.iconpln.co.id/"))
			valCancel()
			if valErr != nil {
				safeLog(fmt.Sprintf("❌ Worker %d: sesi tidak valid: %v", workerID+1, valErr))
				return
			}
			safeLog(fmt.Sprintf("✅ Worker %d: sesi aktif", workerID+1))

			for task := range taskCh {
				if userCtx.Err() != nil {
					return
				}
				err := automator.captureRangeWithRetry(chrome.Ctx, task, maxCaptureRetry, workerID+1, safeLog)
				var sidType string
				if err == nil {
					mu.Lock()
					_, alreadyTyped := sidMeta[task.sid]
					mu.Unlock()
					if !alreadyTyped {
						rawTitle := automator.extractSIDTitle(chrome.Ctx)
						sidType = detectSIDType(rawTitle)
					}
				}
				resultCh <- captureResult{task: task, sidType: sidType, err: err}
			}
		}(i)
	}

	// Tutup resultCh setelah semua worker selesai.
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// Konsumsi hasil — single goroutine, tidak perlu mutex untuk counter.
	for res := range resultCh {
		if res.err != nil {
			logFn(fmt.Sprintf("  ❌ %s: %v", res.task.label, res.err))
			failed++
		} else {
			if res.sidType != "" {
				sidMeta[res.task.sid] = res.sidType
				logFn(fmt.Sprintf("  ✅ %s tersimpan [%s]", res.task.label, res.sidType))
			} else {
				logFn(fmt.Sprintf("  ✅ %s tersimpan", res.task.label))
			}
		}
		done++
		if progFn != nil {
			progFn(done, total)
		}
	}

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
