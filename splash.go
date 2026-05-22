package main

import (
	"image/color"
	"net"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// checkConnectivity verifies TCP reachability to the MRTG server (port 443).
func checkConnectivity() error {
	conn, err := net.DialTimeout("tcp", "mrtg.iconpln.co.id:443", 8*time.Second)
	if err != nil {
		return err
	}
	conn.Close()
	return nil
}

// runSplash shows the splash/connectivity-check screen.
// On success it swaps w's content to mainContent.
func runSplash(a fyne.App, w fyne.Window, mainContent fyne.CanvasObject) {
	// ── Logo + Title ──────────────────────────────────────────────────────────
	logo := canvas.NewImageFromResource(imgResource("pln-logo.png"))
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(130, 130))

	plnTxt := canvas.NewText("PLN", color.NRGBA{R: 0, G: 174, B: 239, A: 255})
	plnTxt.TextStyle = fyne.TextStyle{Bold: true}
	plnTxt.TextSize = 34
	plnTxt.Alignment = fyne.TextAlignCenter

	subTxt := canvas.NewText("Icon Plus", color.NRGBA{R: 120, G: 195, B: 215, A: 255})
	subTxt.TextSize = 16
	subTxt.Alignment = fyne.TextAlignCenter

	appTxt := canvas.NewText("MRTG Chart Automation", color.NRGBA{R: 160, G: 180, B: 210, A: 255})
	appTxt.TextSize = 13
	appTxt.Alignment = fyne.TextAlignCenter

	topSection := container.NewVBox(
		container.NewCenter(logo),
		container.NewCenter(plnTxt),
		container.NewCenter(subTxt),
		container.NewCenter(appTxt),
	)

	// ── Loading section ───────────────────────────────────────────────────────
	activity := widget.NewActivity()

	statusTxt := canvas.NewText("Memeriksa koneksi ke server MRTG...", color.NRGBA{R: 150, G: 170, B: 200, A: 255})
	statusTxt.TextSize = 12
	statusTxt.Alignment = fyne.TextAlignCenter

	loadingSection := container.NewVBox(
		container.NewCenter(activity),
		container.NewCenter(statusTxt),
	)

	// ── Error section ─────────────────────────────────────────────────────────
	warnIcon := canvas.NewText("⚠", color.NRGBA{R: 255, G: 210, B: 0, A: 255})
	warnIcon.TextSize = 52
	warnIcon.Alignment = fyne.TextAlignCenter

	errTitle := canvas.NewText("Koneksi Ke Internet Gagal", colLogErr)
	errTitle.TextStyle = fyne.TextStyle{Bold: true}
	errTitle.TextSize = 18
	errTitle.Alignment = fyne.TextAlignCenter

	errDetail := canvas.NewText("Tidak dapat terhubung ke mrtg.iconpln.co.id", color.NRGBA{R: 130, G: 150, B: 180, A: 255})
	errDetail.TextSize = 11
	errDetail.Alignment = fyne.TextAlignCenter

	retryBtn := widget.NewButton("🔄   Muat Ulang", nil)
	retryBtn.Importance = widget.HighImportance

	exitBtn := widget.NewButton("Keluar", func() { a.Quit() })
	exitBtn.Importance = widget.LowImportance

	errorSection := container.NewVBox(
		container.NewCenter(warnIcon),
		container.NewCenter(errTitle),
		container.NewCenter(errDetail),
		container.NewPadded(container.NewGridWithColumns(2, retryBtn, exitBtn)),
	)
	errorSection.Hide()

	// ── Assembled splash ──────────────────────────────────────────────────────
	splashInner := container.NewVBox(
		container.NewPadded(topSection),
		loadingSection,
		errorSection,
	)

	bg := canvas.NewRectangle(colNavy)
	w.SetContent(container.NewStack(bg, container.NewCenter(splashInner)))

	// ── Connectivity check ────────────────────────────────────────────────────
	var doCheck func()
	doCheck = func() {
		errorSection.Hide()
		loadingSection.Show()
		statusTxt.Text = "Memeriksa koneksi ke server MRTG..."
		canvas.Refresh(statusTxt)
		activity.Start()

		go func() {
			err := checkConnectivity()
			activity.Stop()
			if err != nil {
				loadingSection.Hide()
				errorSection.Show()
			} else {
				w.SetContent(mainContent)
			}
		}()
	}

	retryBtn.OnTapped = func() { doCheck() }
	doCheck()
}
