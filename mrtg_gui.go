package main

import (
	"context"
	"embed"
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ─── Embedded Assets ─────────────────────────────────────────────────────────
//go:embed icon-pln-icon-plus.png pln-logo.png buyme-a-coffee.jpeg
var embedFS embed.FS

func imgResource(name string) fyne.Resource {
	data, err := embedFS.ReadFile(name)
	if err != nil {
		return nil
	}
	return fyne.NewStaticResource(name, data)
}

// ─── Color Palette ────────────────────────────────────────────────────────────
var (
	colNavy    = color.NRGBA{R: 0, G: 51, B: 102, A: 255}
	colNavyHov = color.NRGBA{R: 0, G: 72, B: 140, A: 255}
	colGold    = color.NRGBA{R: 255, G: 215, B: 0, A: 255}
	colGoldHov = color.NRGBA{R: 230, G: 190, B: 0, A: 255}
	colWhite   = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	colBg      = color.NRGBA{R: 243, G: 246, B: 251, A: 255}
	colBorder  = color.NRGBA{R: 218, G: 224, B: 235, A: 255}
	colLogBg   = color.NRGBA{R: 13, G: 17, B: 23, A: 255}
	colLogInfo = color.NRGBA{R: 64, G: 156, B: 255, A: 255}
	colLogOk   = color.NRGBA{R: 0, G: 200, B: 83, A: 255}
	colLogErr  = color.NRGBA{R: 255, G: 82, B: 82, A: 255}
	colLogTime = color.NRGBA{R: 100, G: 110, B: 125, A: 255}
	colLogMsg  = color.NRGBA{R: 195, G: 208, B: 222, A: 255}
	colTxtDark = color.NRGBA{R: 22, G: 33, B: 55, A: 255}
	colTxtGray = color.NRGBA{R: 95, G: 108, B: 128, A: 255}
	colTransp  = color.NRGBA{A: 0}
)

// ─── Custom Theme (PLN) ───────────────────────────────────────────────────────
const (
	clrLogInfo    fyne.ThemeColorName = "logInfo"
	clrLogSuccess fyne.ThemeColorName = "logSuccess"
	clrLogError   fyne.ThemeColorName = "logError"
	clrLogTime    fyne.ThemeColorName = "logTime"
	clrLogMsg     fyne.ThemeColorName = "logMsg"
)

type plnTheme struct{}

func (plnTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	switch n {
	case theme.ColorNamePrimary:
		return colNavy
	case theme.ColorNameBackground:
		return colBg
	case theme.ColorNameInputBackground:
		return colWhite
	case theme.ColorNameForeground:
		return colTxtDark
	case theme.ColorNameSeparator:
		return colBorder
	case clrLogInfo:
		return colLogInfo
	case clrLogSuccess:
		return colLogOk
	case clrLogError:
		return colLogErr
	case clrLogTime:
		return colLogTime
	case clrLogMsg:
		return colLogMsg
	}
	return theme.DefaultTheme().Color(n, v)
}
func (plnTheme) Font(s fyne.TextStyle) fyne.Resource  { return theme.DefaultTheme().Font(s) }
func (plnTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return theme.DefaultTheme().Icon(n) }
func (plnTheme) Size(n fyne.ThemeSizeName) float32       { return theme.DefaultTheme().Size(n) }

// ─── Sidebar Layout (fixed left panel) ───────────────────────────────────────
type sidebarLayout struct{ w float32 }

func (s *sidebarLayout) Layout(obj []fyne.CanvasObject, size fyne.Size) {
	if len(obj) < 2 {
		return
	}
	obj[0].Resize(fyne.NewSize(s.w, size.Height))
	obj[0].Move(fyne.NewPos(0, 0))
	obj[1].Resize(fyne.NewSize(size.Width-s.w, size.Height))
	obj[1].Move(fyne.NewPos(s.w, 0))
}
func (s *sidebarLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(s.w+560, 600)
}

// ─── SelectCard Widget ────────────────────────────────────────────────────────
// Card yang bisa dipilih (navy saat aktif, white saat tidak)
type SelectCard struct {
	widget.BaseWidget
	Icon     string
	Title    string
	Desc     string
	Selected bool
	OnTap    func()
}

func newSelectCard(icon, title, desc string, onTap func()) *SelectCard {
	s := &SelectCard{Icon: icon, Title: title, Desc: desc, OnTap: onTap}
	s.ExtendBaseWidget(s)
	return s
}

func (s *SelectCard) SetSelected(v bool) { s.Selected = v; s.Refresh() }

func (s *SelectCard) Tapped(*fyne.PointEvent) {
	if s.OnTap != nil {
		s.OnTap()
	}
}

func (s *SelectCard) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(colWhite)
	bg.CornerRadius = 10
	bg.StrokeColor = colBorder
	bg.StrokeWidth = 1.5

	check := canvas.NewText("○", colBorder)
	check.TextSize = 18

	icon := canvas.NewText(s.Icon, colNavy)
	icon.TextSize = 30
	icon.Alignment = fyne.TextAlignCenter

	title := canvas.NewText(s.Title, colTxtDark)
	title.TextStyle = fyne.TextStyle{Bold: true}
	title.TextSize = 13
	title.Alignment = fyne.TextAlignCenter

	desc := canvas.NewText(s.Desc, colTxtGray)
	desc.TextSize = 11
	desc.Alignment = fyne.TextAlignCenter

	r := &selectCardRenderer{
		card:    s,
		bg:      bg,
		check:   check,
		icon:    icon,
		title:   title,
		desc:    desc,
		objects: []fyne.CanvasObject{bg, check, icon, title, desc},
	}
	r.Refresh()
	return r
}

type selectCardRenderer struct {
	card             *SelectCard
	bg               *canvas.Rectangle
	check, icon      *canvas.Text
	title, desc      *canvas.Text
	objects          []fyne.CanvasObject
}

func (r *selectCardRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *selectCardRenderer) Destroy()                     {}
func (r *selectCardRenderer) MinSize() fyne.Size           { return fyne.NewSize(145, 130) }

func (r *selectCardRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	r.check.Resize(fyne.NewSize(22, 22))
	r.check.Move(fyne.NewPos(size.Width-28, 6))

	iconH := float32(38)
	r.icon.Resize(fyne.NewSize(size.Width, iconH))
	r.icon.Move(fyne.NewPos(0, 18))

	titleH := float32(20)
	r.title.Resize(fyne.NewSize(size.Width-16, titleH))
	r.title.Move(fyne.NewPos(8, 18+iconH+4))

	r.desc.Resize(fyne.NewSize(size.Width-16, 16))
	r.desc.Move(fyne.NewPos(8, 18+iconH+4+titleH+2))
}

func (r *selectCardRenderer) Refresh() {
	if r.card.Selected {
		r.bg.FillColor = colNavy
		r.bg.StrokeColor = colNavy
		r.icon.Color = colWhite
		r.title.Color = colWhite
		r.desc.Color = color.NRGBA{R: 180, G: 195, B: 215, A: 255}
		r.check.Text = "✓"
		r.check.Color = colGold
	} else {
		r.bg.FillColor = colWhite
		r.bg.StrokeColor = colBorder
		r.icon.Color = colNavy
		r.title.Color = colTxtDark
		r.desc.Color = colTxtGray
		r.check.Text = "○"
		r.check.Color = colBorder
	}
	canvas.Refresh(r.bg)
	canvas.Refresh(r.check)
	canvas.Refresh(r.icon)
	canvas.Refresh(r.title)
	canvas.Refresh(r.desc)
}

// ─── NavItem Widget ───────────────────────────────────────────────────────────
type navItem struct {
	widget.BaseWidget
	icon, label string
	active      bool
	onTap       func()
}

func newNavItem(icon, label string, active bool, onTap func()) *navItem {
	n := &navItem{icon: icon, label: label, active: active, onTap: onTap}
	n.ExtendBaseWidget(n)
	return n
}

func (n *navItem) Tapped(*fyne.PointEvent) {
	if n.onTap != nil {
		n.onTap()
	}
}

func (n *navItem) CreateRenderer() fyne.WidgetRenderer {
	var bgCol color.Color = colTransp
	if n.active {
		bgCol = colNavyHov
	}
	bg := canvas.NewRectangle(bgCol)
	bg.CornerRadius = 8

	ico := canvas.NewText(n.icon, colWhite)
	ico.TextSize = 16

	lbl := canvas.NewText(n.label, colWhite)
	lbl.TextSize = 13

	return &navItemRenderer{bg: bg, ico: ico, lbl: lbl,
		objects: []fyne.CanvasObject{bg, ico, lbl}}
}

type navItemRenderer struct {
	bg            *canvas.Rectangle
	ico, lbl      *canvas.Text
	objects       []fyne.CanvasObject
}

func (r *navItemRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *navItemRenderer) Destroy()                     {}
func (r *navItemRenderer) MinSize() fyne.Size           { return fyne.NewSize(180, 40) }

func (r *navItemRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	r.ico.Resize(fyne.NewSize(22, size.Height))
	r.ico.Move(fyne.NewPos(14, 0))
	r.lbl.Resize(fyne.NewSize(size.Width-50, size.Height))
	r.lbl.Move(fyne.NewPos(44, 0))
}

func (r *navItemRenderer) Refresh() {}

// ─── Log Console Widget ────────────────────────────────────────────────────────
// Widget log dengan background gelap dan teks berwarna
type logConsole struct {
	widget.BaseWidget
	lines   []*logLine
	binding []fyne.CanvasObject
}

type logLine struct {
	timestamp, level, message string
}

func newLogConsole() *logConsole {
	l := &logConsole{}
	l.ExtendBaseWidget(l)
	return l
}

func (l *logConsole) AddLine(timestamp, level, message string) {
	l.lines = append(l.lines, &logLine{timestamp, level, message})
	l.Refresh()
}

func (l *logConsole) Clear() {
	l.lines = nil
	l.Refresh()
}

func (l *logConsole) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(colLogBg)
	return &logRenderer{console: l, bg: bg}
}

type logRenderer struct {
	console  *logConsole
	bg       *canvas.Rectangle
	rowObjs  []fyne.CanvasObject
	size     fyne.Size
}

func (r *logRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg}
	return append(objs, r.rowObjs...)
}

func (r *logRenderer) Destroy() {}

func (r *logRenderer) MinSize() fyne.Size {
	return fyne.NewSize(400, 180)
}

func (r *logRenderer) Layout(size fyne.Size) {
	r.size = size
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))

	lineH := float32(18)
	pad := float32(8)
	y := pad

	maxRows := int((size.Height - pad*2) / lineH)
	start := 0
	if len(r.rowObjs)/3 > maxRows {
		start = (len(r.rowObjs)/3 - maxRows) * 3
	}

	for i := start; i < len(r.rowObjs); i += 3 {
		if i+2 >= len(r.rowObjs) {
			break
		}
		ts := r.rowObjs[i]
		lv := r.rowObjs[i+1]
		msg := r.rowObjs[i+2]

		ts.Resize(fyne.NewSize(80, lineH))
		ts.Move(fyne.NewPos(pad, y))

		lv.Resize(fyne.NewSize(75, lineH))
		lv.Move(fyne.NewPos(pad+80, y))

		msg.Resize(fyne.NewSize(size.Width-pad*2-155, lineH))
		msg.Move(fyne.NewPos(pad+155, y))
		y += lineH
	}
}

func (r *logRenderer) Refresh() {
	r.rowObjs = nil
	for _, line := range r.console.lines {
		var lvCol color.Color
		switch line.level {
		case "INFO":
			lvCol = colLogInfo
		case "SUCCESS":
			lvCol = colLogOk
		case "ERROR":
			lvCol = colLogErr
		default:
			lvCol = colLogMsg
		}

		ts := canvas.NewText(line.timestamp, colLogTime)
		ts.TextSize = 11

		lv := canvas.NewText("["+line.level+"]", lvCol)
		lv.TextSize = 11

		msg := canvas.NewText(line.message, colLogMsg)
		msg.TextSize = 11

		r.rowObjs = append(r.rowObjs, ts, lv, msg)
	}
	r.bg.FillColor = colLogBg
	r.Layout(r.size)
	canvas.Refresh(r.bg)
	for _, o := range r.rowObjs {
		canvas.Refresh(o)
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func sectionHeader(num, icon, title string) fyne.CanvasObject {
	numTxt := canvas.NewText(num+".", colNavy)
	numTxt.TextStyle = fyne.TextStyle{Bold: true}
	numTxt.TextSize = 13

	icoTxt := canvas.NewText(icon, colNavy)
	icoTxt.TextSize = 15

	titleTxt := canvas.NewText(title, colTxtDark)
	titleTxt.TextStyle = fyne.TextStyle{Bold: true}
	titleTxt.TextSize = 14

	row := container.NewHBox(
		container.NewPadded(numTxt),
		icoTxt,
		container.NewPadded(titleTxt),
	)

	sep := canvas.NewRectangle(colBorder)
	sep.SetMinSize(fyne.NewSize(0, 1))

	return container.NewVBox(row, sep)
}

func cardBox(content fyne.CanvasObject) fyne.CanvasObject {
	bg := canvas.NewRectangle(colWhite)
	bg.CornerRadius = 12
	bg.StrokeColor = colBorder
	bg.StrokeWidth = 1

	inner := container.NewPadded(container.NewPadded(content))
	return container.NewStack(bg, inner)
}

// segmentedToggle membuat dua tombol yang berfungsi seperti tab/toggle
func segmentedToggle(labels [2]string, onSelect func(int)) [2]*widget.Button {
	var btns [2]*widget.Button
	btns[0] = widget.NewButton(labels[0], func() {
		btns[0].Importance = widget.HighImportance
		btns[1].Importance = widget.LowImportance
		btns[0].Refresh()
		btns[1].Refresh()
		onSelect(0)
	})
	btns[1] = widget.NewButton(labels[1], func() {
		btns[1].Importance = widget.HighImportance
		btns[0].Importance = widget.LowImportance
		btns[0].Refresh()
		btns[1].Refresh()
		onSelect(1)
	})
	btns[0].Importance = widget.HighImportance
	btns[1].Importance = widget.LowImportance
	return btns
}

// ─── Main GUI ─────────────────────────────────────────────────────────────────
func launchGUI() {
	a := app.NewWithID("com.pln.mrtg")
	a.Settings().SetTheme(plnTheme{})
	a.SetIcon(imgResource("pln-logo.png"))
	w := a.NewWindow("MRTG Chart Automation - PLN Icon Plus")
	w.Resize(fyne.NewSize(1180, 820))

	runSplash(a, w, buildMainLayout(w))
	w.Show()
	maximizeOnStart()
	a.Run()
}

func buildMainLayout(w fyne.Window) fyne.CanvasObject {
	// Start telemetry heartbeat (non-blocking).
	startTelemetry(w)

	// ── Sidebar ──────────────────────────────────────────────────────────────
	sidebarBg := canvas.NewRectangle(colNavy)

	// Logo sidebar — pln-logo.png (kotak kuning dengan petir)
	sideLogoImg := canvas.NewImageFromResource(imgResource("pln-logo.png"))
	sideLogoImg.FillMode = canvas.ImageFillContain
	sideLogoImg.SetMinSize(fyne.NewSize(72, 72))
	logoArea := container.NewCenter(sideLogoImg)

	// Teks "PLN" — biru cerah brand PLN
	plnTitle := canvas.NewText("PLN", color.NRGBA{R: 0, G: 174, B: 239, A: 255})
	plnTitle.TextStyle = fyne.TextStyle{Bold: true}
	plnTitle.TextSize = 22
	plnTitle.Alignment = fyne.TextAlignCenter

	// Teks "Icon Plus" — teal terang
	plnSub := canvas.NewText("Icon Plus", color.NRGBA{R: 120, G: 195, B: 215, A: 255})
	plnSub.TextSize = 12
	plnSub.Alignment = fyne.TextAlignCenter

	logoSection := container.NewVBox(
		container.NewCenter(logoArea),
		container.NewCenter(plnTitle),
		container.NewCenter(plnSub),
	)

	sep := canvas.NewRectangle(color.NRGBA{R: 255, G: 255, B: 255, A: 30})
	sep.SetMinSize(fyne.NewSize(0, 1))

	navDash := newNavItem("🏠", "Dashboard", true, nil)
	navHistory := newNavItem("🕐", "Riwayat Capture", false, func() {
		showHistoryDialog(w)
	})
	navSettings := newNavItem("⚙", "Pengaturan", false, func() {
		dialog.ShowInformation("Info", "Fitur akan segera hadir", w)
	})
	navAbout := newNavItem("ⓘ", "Tentang Aplikasi", false, func() {
		showAboutDialog(w)
	})

	verLabel := canvas.NewText("Versi 1.0.0", color.NRGBA{R: 150, G: 170, B: 200, A: 255})
	verLabel.TextSize = 11
	verLabel.Alignment = fyne.TextAlignCenter

	creditName := canvas.NewText("ZIAUL KAMAL", color.NRGBA{R: 180, G: 195, B: 215, A: 255})
	creditName.TextStyle = fyne.TextStyle{Bold: true}
	creditName.TextSize = 10
	creditName.Alignment = fyne.TextAlignCenter

	creditUnit := canvas.NewText("EOS Aceh Barat Daya", color.NRGBA{R: 130, G: 150, B: 175, A: 255})
	creditUnit.TextSize = 9
	creditUnit.Alignment = fyne.TextAlignCenter

	// Tipis pemisah emas sebelum tombol donasi
	coffSep := canvas.NewRectangle(color.NRGBA{R: 255, G: 215, B: 0, A: 45})
	coffSep.SetMinSize(fyne.NewSize(0, 1))

	navCoffee := newNavItem("☕", "Buy Me a Coffee", false, func() {
		showCoffeeDialog(w)
	})

	sidebarContent := container.NewVBox(
		container.NewPadded(logoSection),
		container.NewPadded(sep),
		container.NewPadded(navDash),
		container.NewPadded(navHistory),
		container.NewPadded(navSettings),
		container.NewPadded(navAbout),
		container.NewPadded(coffSep),
		container.NewPadded(navCoffee),
		widget.NewLabel(""), // spacer
	)

	// ── User count widget ────────────────────────────────────────────────────
	userCountTxt := canvas.NewText("👥 — aktif", color.NRGBA{R: 130, G: 200, B: 130, A: 255})
	userCountTxt.TextSize = 11
	userCountTxt.Alignment = fyne.TextAlignCenter

	userCountBtn := widget.NewButton("", func() {
		showMasterPanelPrompt(w)
	})
	userCountBtn.Importance = widget.LowImportance

	userCountStack := container.NewStack(
		userCountBtn,
		container.NewCenter(userCountTxt),
	)

	// Refresh user count label every 30 seconds.
	go func() {
		for {
			n := GetActiveCount()
			label := "👥 — aktif"
			if n >= 0 {
				label = fmt.Sprintf("👥 %d aktif", n)
			}
			fyne.Do(func() { userCountTxt.Text = label; userCountTxt.Refresh() })
			time.Sleep(30 * time.Second)
		}
	}()

	sidebarFooter := container.NewVBox(
		container.NewCenter(userCountStack),
		container.NewCenter(verLabel),
		container.NewCenter(creditName),
		container.NewCenter(creditUnit),
	)

	sidebar := container.NewStack(
		sidebarBg,
		container.NewBorder(nil, container.NewPadded(sidebarFooter), nil, nil, sidebarContent),
	)

	// ── Main Header ──────────────────────────────────────────────────────────
	mainTitle := canvas.NewText("MRTG Chart Automation", colNavy)
	mainTitle.TextStyle = fyne.TextStyle{Bold: true}
	mainTitle.TextSize = 26

	mainSub := canvas.NewText("Automasi capture grafik MRTG untuk monitoring jaringan PLN", colTxtGray)
	mainSub.TextSize = 12

	// Header logo kanan — icon-pln-icon-plus.png
	headerLogoImg := canvas.NewImageFromResource(imgResource("icon-pln-icon-plus.png"))
	headerLogoImg.FillMode = canvas.ImageFillContain
	headerLogoImg.SetMinSize(fyne.NewSize(170, 48))

	headerLeft := container.NewVBox(mainTitle, mainSub)
	mainHeader := container.NewBorder(nil, nil, nil,
		container.NewCenter(headerLogoImg),
		headerLeft,
	)

	// ── Card 1: Login ─────────────────────────────────────────────────────────
	loginMethodIdx := 0 // 0 = SID, 1 = Username/Pass

	sidLoginEntry := widget.NewEntry()
	sidLoginEntry.SetPlaceHolder("Masukkan SID Anda")

	usernameEntry := widget.NewEntry()
	usernameEntry.SetPlaceHolder("Masukkan username Anda")
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetPlaceHolder("Masukkan password Anda")

	sidForm := container.NewVBox(
		widget.NewLabel("SID"),
		sidLoginEntry,
	)
	userForm := container.NewVBox(
		widget.NewLabel("Username"),
		usernameEntry,
		widget.NewLabel("Password"),
		passwordEntry,
	)
	userForm.Hide()

	loginBtns := segmentedToggle([2]string{"  👤  SID  ", "  👤  Username  "}, func(idx int) {
		loginMethodIdx = idx
		if idx == 0 {
			sidForm.Show()
			userForm.Hide()
		} else {
			sidForm.Hide()
			userForm.Show()
		}
	})

	loginCard := cardBox(container.NewVBox(
		sectionHeader("1", "🔐", "Login"),
		container.NewPadded(container.NewGridWithColumns(2, loginBtns[0], loginBtns[1])),
		container.NewPadded(sidForm),
		container.NewPadded(userForm),
	))

	// ── Card 2: Capture Mode ──────────────────────────────────────────────────
	captureModeIdx := 0 // 0 = single, 1 = multiple

	sidCaptureEntry := widget.NewEntry()
	sidCaptureEntry.SetPlaceHolder("Contoh: 231401002368")

	fileEntry := widget.NewEntry()
	fileEntry.SetPlaceHolder("Path file SID...")
	browseFileBtn := widget.NewButton("Browse", nil)
	browseFileBtn.OnTapped = func() {
		pickFile("Pilih File SID", w, func(p string) {
			fileEntry.SetText(p)
		})
	}

	sidInputBox := container.NewVBox(
		widget.NewLabel("SID Layanan"),
		sidCaptureEntry,
	)
	fileInputBox := container.NewVBox(
		widget.NewLabel("File SID (satu SID per baris, # untuk komentar)"),
		container.NewBorder(nil, nil, nil, browseFileBtn, fileEntry),
	)
	fileInputBox.Hide()

	var cardSingle, cardMultiple *SelectCard
	cardSingle = newSelectCard("🎯", "Single Capture", "Capture satu target", func() {
		captureModeIdx = 0
		cardSingle.SetSelected(true)
		cardMultiple.SetSelected(false)
		sidInputBox.Show()
		fileInputBox.Hide()
	})
	cardMultiple = newSelectCard("📋", "Multiple Capture", "Capture banyak target", func() {
		captureModeIdx = 1
		cardMultiple.SetSelected(true)
		cardSingle.SetSelected(false)
		sidInputBox.Hide()
		fileInputBox.Show()
	})
	cardSingle.SetSelected(true)

	captureCard := cardBox(container.NewVBox(
		sectionHeader("2", "📷", "Capture Mode"),
		container.NewPadded(container.NewGridWithColumns(2, cardSingle, cardMultiple)),
		container.NewPadded(sidInputBox),
		container.NewPadded(fileInputBox),
	))

	// ── Card 3: Date Range ────────────────────────────────────────────────────
	dateRangeIdx := 0

	monthEntry := widget.NewEntry()
	monthEntry.SetPlaceHolder("YYYY/MM — contoh: 2026/06")
	monthCalBtn := widget.NewButton("📅", func() {
		t, _ := time.Parse("2006/01", strings.TrimSpace(monthEntry.Text))
		if t.IsZero() {
			t = time.Now()
		}
		showMonthPicker(t, func(sel time.Time) {
			monthEntry.SetText(sel.Format("2006/01"))
		}, w)
	})
	monthCalBtn.Importance = widget.LowImportance
	monthBox := container.NewVBox(
		widget.NewLabel("Bulan"),
		container.NewBorder(nil, nil, nil, monthCalBtn, monthEntry),
	)

	startDateEntry := widget.NewEntry()
	startDateEntry.SetPlaceHolder("YYYY/MM/DD — contoh: 2026/06/05")
	startCalBtn := widget.NewButton("📅", func() {
		t, _ := time.Parse("2006/01/02", strings.TrimSpace(startDateEntry.Text))
		if t.IsZero() {
			t = time.Now()
		}
		showDatePicker(t, func(sel time.Time) {
			startDateEntry.SetText(sel.Format("2006/01/02"))
		}, w)
	})
	startCalBtn.Importance = widget.LowImportance

	endDateEntry := widget.NewEntry()
	endDateEntry.SetPlaceHolder("YYYY/MM/DD — contoh: 2026/06/20")
	endCalBtn := widget.NewButton("📅", func() {
		t, _ := time.Parse("2006/01/02", strings.TrimSpace(endDateEntry.Text))
		if t.IsZero() {
			t = time.Now()
		}
		showDatePicker(t, func(sel time.Time) {
			endDateEntry.SetText(sel.Format("2006/01/02"))
		}, w)
	})
	endCalBtn.Importance = widget.LowImportance

	rangeBox := container.NewVBox(
		widget.NewLabel("Tanggal Mulai"),
		container.NewBorder(nil, nil, nil, startCalBtn, startDateEntry),
		widget.NewLabel("Tanggal Akhir"),
		container.NewBorder(nil, nil, nil, endCalBtn, endDateEntry),
	)
	rangeBox.Hide()

	var cardFullMonth, cardCustomRange *SelectCard
	cardFullMonth = newSelectCard("📊", "Laporan Bulanan", "1 file/SID • grafik 1 bulan", func() {
		dateRangeIdx = 0
		cardFullMonth.SetSelected(true)
		cardCustomRange.SetSelected(false)
		monthBox.Show()
		rangeBox.Hide()
	})
	cardCustomRange = newSelectCard("📅", "Laporan Harian", "1 file/hari per SID", func() {
		dateRangeIdx = 1
		cardCustomRange.SetSelected(true)
		cardFullMonth.SetSelected(false)
		monthBox.Hide()
		rangeBox.Show()
	})
	cardFullMonth.SetSelected(true)

	dateCard := cardBox(container.NewVBox(
		sectionHeader("3", "📅", "Jenis Laporan"),
		container.NewPadded(container.NewGridWithColumns(2, cardFullMonth, cardCustomRange)),
		container.NewPadded(monthBox),
		container.NewPadded(rangeBox),
	))

	// ── Card 4: Output ────────────────────────────────────────────────────────
	outputEntry := widget.NewEntry()
	outputEntry.SetPlaceHolder("C:\\MRTG_Output  atau  ./output")
	browseDirBtn := widget.NewButton("📁  Pilih Folder", nil)
	browseDirBtn.Importance = widget.MediumImportance
	browseDirBtn.OnTapped = func() {
		pickFolder("Pilih Folder Output", w, func(p string) {
			outputEntry.SetText(p)
		})
	}

	outputCard := cardBox(container.NewVBox(
		sectionHeader("4", "💾", "Output Folder"),
		container.NewPadded(container.NewVBox(
			widget.NewLabel("Folder penyimpanan hasil capture"),
			container.NewBorder(nil, nil, nil, browseDirBtn, outputEntry),
		)),
	))

	// ── Progress & Run Button ─────────────────────────────────────────────────
	progressBar := widget.NewProgressBar()
	progressBar.Hide()

	pctLabel := canvas.NewText("0%", colTxtDark)
	pctLabel.TextStyle = fyne.TextStyle{Bold: true}
	pctLabel.TextSize = 16
	pctLabel.Alignment = fyne.TextAlignTrailing

	progressLabel := canvas.NewText("Progress", colTxtDark)
	progressLabel.TextStyle = fyne.TextStyle{Bold: true}
	progressLabel.TextSize = 13

	progressSection := container.NewBorder(nil, nil,
		container.NewVBox(progressLabel),
		container.NewVBox(pctLabel),
		progressBar,
	)

	// Gold "Mulai Capture" button
	runBtn := widget.NewButton("▶   Mulai Capture", nil)
	runBtn.Importance = widget.HighImportance

	// Tombol Batalkan — hanya muncul saat capture berjalan
	cancelBtn := widget.NewButton("⛔   Batalkan", nil)
	cancelBtn.Importance = widget.DangerImportance
	cancelBtn.Hide()

	btnArea := container.NewGridWithColumns(2, runBtn, cancelBtn)

	bottomRow := container.NewGridWithColumns(2,
		container.NewPadded(btnArea),
		container.NewPadded(progressSection),
	)

	// ── Log Console ───────────────────────────────────────────────────────────
	logCons := newLogConsole()
	logScroll := container.NewVScroll(logCons)
	logScroll.SetMinSize(fyne.NewSize(0, 200))

	clearLogBtn := widget.NewButton("🗑  Bersihkan Log", func() {
		logCons.Clear()
	})
	clearLogBtn.Importance = widget.LowImportance

	logHeaderTxt := canvas.NewText("5.  Log Console", colWhite)
	logHeaderTxt.TextStyle = fyne.TextStyle{Bold: true}
	logHeaderTxt.TextSize = 13

	logHeaderBg := canvas.NewRectangle(color.NRGBA{R: 22, G: 30, B: 46, A: 255})
	logHeaderBg.CornerRadius = 0

	logHeader := container.NewStack(
		logHeaderBg,
		container.NewBorder(nil, nil,
			container.NewPadded(logHeaderTxt),
			container.NewPadded(clearLogBtn),
			nil,
		),
	)

	logCardBg := canvas.NewRectangle(colLogBg)
	logCardBg.CornerRadius = 12
	logCardBg.StrokeColor = color.NRGBA{R: 40, G: 50, B: 65, A: 255}
	logCardBg.StrokeWidth = 1

	logSection := container.NewStack(
		logCardBg,
		container.NewVBox(logHeader, logScroll),
	)

	appendLog := func(msg string) {
		ts := time.Now().Format("15:04:05")
		level := "INFO"
		clean := msg
		if strings.Contains(msg, "✅") || strings.Contains(msg, "tersimpan") || strings.Contains(msg, "berhasil") {
			level = "SUCCESS"
			clean = strings.ReplaceAll(msg, "✅ ", "")
		} else if strings.Contains(msg, "❌") || strings.Contains(msg, "gagal") || strings.Contains(msg, "Gagal") {
			level = "ERROR"
			clean = strings.ReplaceAll(msg, "❌ ", "")
		}
		clean = strings.TrimSpace(strings.ReplaceAll(clean, "\n", " "))
		if clean == "" {
			return
		}
		logCons.AddLine(ts, level, clean)
		logScroll.ScrollToBottom()
	}

	// ── State pembatalan & close intercept ───────────────────────────────────
	isCapturing := false
	var currentCancel context.CancelFunc

	// ── Run Logic ─────────────────────────────────────────────────────────────
	runBtn.OnTapped = func() {
		cfg := &CaptureConfig{}

		// Validasi login
		if loginMethodIdx == 0 {
			v := strings.TrimSpace(sidLoginEntry.Text)
			if v == "" {
				dialog.ShowError(fmt.Errorf("SID login tidak boleh kosong"), w)
				return
			}
			cfg.UseSID = true
			cfg.LoginSID = v
		} else {
			u := strings.TrimSpace(usernameEntry.Text)
			p := strings.TrimSpace(passwordEntry.Text)
			if u == "" || p == "" {
				dialog.ShowError(fmt.Errorf("Username dan password tidak boleh kosong"), w)
				return
			}
			cfg.UseSID = false
			cfg.Username = u
			cfg.Password = p
		}

		// Validasi SID
		if captureModeIdx == 0 {
			s := strings.TrimSpace(sidCaptureEntry.Text)
			if s == "" {
				dialog.ShowError(fmt.Errorf("SID Layanan tidak boleh kosong"), w)
				return
			}
			cfg.SIDs = []string{s}
		} else {
			fp := strings.TrimSpace(fileEntry.Text)
			if fp == "" {
				dialog.ShowError(fmt.Errorf("Path file SID tidak boleh kosong"), w)
				return
			}
			sids, err := readSIDsFromFile(fp)
			if err != nil {
				dialog.ShowError(fmt.Errorf("Gagal baca file:\n%v", err), w)
				return
			}
			if len(sids) == 0 {
				dialog.ShowError(fmt.Errorf("File SID kosong"), w)
				return
			}
			cfg.SIDs = sids
		}

		// Validasi tanggal
		if dateRangeIdx == 0 {
			t, err := time.Parse("2006/01", strings.TrimSpace(monthEntry.Text))
			if err != nil {
				dialog.ShowError(fmt.Errorf("Format bulan tidak valid\nGunakan YYYY/MM, contoh: 2026/06"), w)
				return
			}
			cfg.IsMonthly = true
			cfg.StartDate = time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.Local)
			now := time.Now()
			if t.Year() == now.Year() && t.Month() == now.Month() {
				cfg.EndDate = now
			} else {
				cfg.EndDate = time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.Local)
			}
		} else {
			s, e1 := time.Parse("2006/01/02", strings.TrimSpace(startDateEntry.Text))
			e, e2 := time.Parse("2006/01/02", strings.TrimSpace(endDateEntry.Text))
			if e1 != nil || e2 != nil {
				dialog.ShowError(fmt.Errorf("Format tanggal tidak valid\nGunakan YYYY/MM/DD, contoh: 2026/06/05"), w)
				return
			}
			if e.Before(s) {
				dialog.ShowError(fmt.Errorf("Tanggal akhir tidak boleh sebelum tanggal mulai"), w)
				return
			}
			cfg.IsMonthly = false
			cfg.StartDate = s
			cfg.EndDate = e
		}

		baseDir := strings.TrimSpace(outputEntry.Text)
		if baseDir == "" {
			baseDir = "./output"
		}
		modeLabel := "Harian"
		if cfg.IsMonthly {
			modeLabel = "Bulanan"
		}
		cfg.OutputDir = filepath.Join(baseDir, fmt.Sprintf("capture_%s_%s", time.Now().Format("02_01_2006"), modeLabel))

		// Hitung total file untuk progress & riwayat
		var total int
		var summaryStr string
		if cfg.IsMonthly {
			total = len(cfg.SIDs)
			summaryStr = fmt.Sprintf(
				"📊 Laporan Bulanan\n🎯 SID: %d  •  Total: %d file\n📅 %s – %s\n📁 %s",
				len(cfg.SIDs), total,
				cfg.StartDate.Format("02/01/2006"), cfg.EndDate.Format("02/01/2006"),
				cfg.OutputDir,
			)
		} else {
			dates := generateDateRange(cfg.StartDate, cfg.EndDate)
			total = len(cfg.SIDs) * len(dates)
			summaryStr = fmt.Sprintf(
				"📅 Laporan Harian\n🎯 SID: %d  •  Hari: %d  •  Total: %d file\n📅 %s – %s\n📁 %s",
				len(cfg.SIDs), len(dates), total,
				cfg.StartDate.Format("02/01/2006"), cfg.EndDate.Format("02/01/2006"),
				cfg.OutputDir,
			)
		}

		// ── Dialog Mode Proses ────────────────────────────────────────────────
		selectedWorkers := 1

		workerOpts := []string{
			"1", "2", "3  (default)", "4", "5",
			"6", "7", "8", "9", "10",
		}
		workerSel := widget.NewSelect(workerOpts, func(_ string) {})
		workerSel.SetSelectedIndex(2) // default 3

		workerBox := container.NewVBox(
			widget.NewLabel("Jumlah Worker (Chrome paralel)"),
			workerSel,
		)
		workerBox.Hide()

		var modeNormalCard, modeMultiCard *SelectCard
		modeNormalCard = newSelectCard("🔄", "Normal", "1 Chrome, paling stabil", func() {
			selectedWorkers = 1
			modeNormalCard.SetSelected(true)
			modeMultiCard.SetSelected(false)
			workerBox.Hide()
		})
		modeMultiCard = newSelectCard("⚡", "Multi Process", "Chrome paralel, lebih cepat", func() {
			modeMultiCard.SetSelected(true)
			modeNormalCard.SetSelected(false)
			workerBox.Show()
			selectedWorkers = workerSel.SelectedIndex() + 1
		})
		modeNormalCard.SetSelected(true)

		workerSel.OnChanged = func(_ string) {
			if modeMultiCard.Selected {
				selectedWorkers = workerSel.SelectedIndex() + 1
			}
		}

		sep2 := canvas.NewRectangle(colBorder)
		sep2.SetMinSize(fyne.NewSize(0, 1))

		summaryLbl := widget.NewLabel(summaryStr)
		summaryLbl.Wrapping = fyne.TextWrapWord

		modeContent := container.NewVBox(
			container.NewGridWithColumns(2, modeNormalCard, modeMultiCard),
			container.NewPadded(workerBox),
			container.NewPadded(sep2),
			container.NewPadded(summaryLbl),
		)

		// Goroutine capture — dipanggil setelah mode dipilih dan dikonfirmasi
		launchCapture := func() {
			cfg.Workers = selectedWorkers

			captureCtx, captureCancel := context.WithCancel(context.Background())
			currentCancel = captureCancel
			isCapturing = true

			cancelBtn.Enable()
			cancelBtn.Show()
			cancelBtn.OnTapped = func() {
				captureCancel()
				cancelBtn.Disable()
				appendLog("⛔ Membatalkan capture, harap tunggu...")
			}

			runBtn.Disable()
			progressBar.Show()
			progressBar.SetValue(0)
			pctLabel.Text = "0%"
			canvas.Refresh(pctLabel)
			logCons.Clear()

			outDir := cfg.OutputDir
			go func() {
				defer func() {
					captureCancel()
					isCapturing = false
					currentCancel = nil
					cancelBtn.Hide()
					runBtn.Enable()
				}()

				captureErr := runCapture(captureCtx, cfg,
					func(logMsg string) { appendLog(logMsg) },
					func(done, tot int) {
						pct := float64(done) / float64(tot)
						progressBar.SetValue(pct)
						pctLabel.Text = fmt.Sprintf("%.0f%%", pct*100)
						canvas.Refresh(pctLabel)
					},
				)

				wasCancelled := captureErr == errCancelled
				if captureErr != nil && !wasCancelled {
					appendLog("❌ Error: " + captureErr.Error())
				}

				if wasCancelled {
					progressBar.SetValue(0)
					pctLabel.Text = "—"
				} else {
					progressBar.SetValue(1)
					pctLabel.Text = "100%"
				}
				canvas.Refresh(pctLabel)

				appendHistoryEntry(HistoryEntry{
					Timestamp:    nowTimestamp(),
					OutputFolder: outDir,
					SIDCount:     len(cfg.SIDs),
					DateRange: cfg.StartDate.Format("02/01/2006") + " – " +
						cfg.EndDate.Format("02/01/2006"),
					TotalFiles: total,
					HasError:   captureErr != nil && !wasCancelled,
					Cancelled:  wasCancelled,
					IsMonthly:  cfg.IsMonthly,
					SIDs:       cfg.SIDs,
				})

				if !wasCancelled {
					if captureErr != nil {
						// Ada error — hanya tawarkan buka folder
						dialog.ShowConfirm("Selesai dengan Error",
							"Capture selesai dengan beberapa error.\n\nBuka folder hasil?\n"+outDir,
							func(open bool) {
								if open {
									openOutputFolder(outDir)
								}
							}, w)
					} else {
						// Sukses penuh — tawarkan generate laporan sekarang
						modeTxt := "Harian"
						if cfg.IsMonthly {
							modeTxt = "Bulanan"
						}
						openFolderBtn := widget.NewButton("📂  Buka Folder", func() {
							openOutputFolder(outDir)
						})
						openFolderBtn.Importance = widget.LowImportance

						genLaporanBtn := widget.NewButton("📄  Generate Laporan "+modeTxt, nil)
						genLaporanBtn.Importance = widget.HighImportance

						successIcon := canvas.NewText("✅", colLogOk)
						successIcon.TextSize = 48
						successIcon.Alignment = fyne.TextAlignCenter

						msgTxt := canvas.NewText(
							fmt.Sprintf("Semua capture selesai!  %d SID • %d file", len(cfg.SIDs), total),
							colTxtGray,
						)
						msgTxt.TextSize = 13
						msgTxt.Alignment = fyne.TextAlignCenter

						folderTxt := canvas.NewText("📁  "+outDir, colNavy)
						folderTxt.TextSize = 11
						folderTxt.Alignment = fyne.TextAlignCenter

						btnRow := container.NewGridWithColumns(2, openFolderBtn, genLaporanBtn)
						content := container.NewVBox(
							container.NewCenter(successIcon),
							container.NewCenter(msgTxt),
							container.NewCenter(folderTxt),
							widget.NewSeparator(),
							container.NewPadded(btnRow),
						)

						var successDlg dialog.Dialog
						successDlg = dialog.NewCustom("🎉  Capture Selesai", "Tutup", content, w)
						successDlg.Resize(fyne.NewSize(520, 260))

						genLaporanBtn.OnTapped = func() {
							successDlg.Hide()
							pickDocx("Pilih Template DOCX (Laporan "+modeTxt+")", w, func(templatePath string) {
								if templatePath == "" {
									return
								}

								doGenerate := func(ticketData string) {
									var reportName string
									if cfg.IsMonthly {
										reportName = fmt.Sprintf("Laporan_Bulanan_%s.docx",
											cfg.StartDate.Format("01-2006"))
									} else {
										reportName = fmt.Sprintf("Laporan_Harian_%s_%s.docx",
											cfg.StartDate.Format("02-01-2006"),
											cfg.EndDate.Format("02-01-2006"))
									}
									outputDocx := filepath.Join(outDir, reportName)
									rcfg := &ReportGenConfig{
										TemplatePath: templatePath,
										OutputPath:   outputDocx,
										IsMonthly:    cfg.IsMonthly,
										SIDs:         cfg.SIDs,
										ImageDir:     outDir,
										StartDate:    cfg.StartDate,
										EndDate:      cfg.EndDate,
										TicketData:   ticketData,
									}
									go func() {
										if err := GenerateReport(rcfg); err != nil {
											fyne.Do(func() {
												dialog.ShowError(fmt.Errorf("Gagal generate laporan:\n%v", err), w)
											})
											return
										}
										fyne.Do(func() {
											dialog.ShowConfirm(
												"Laporan Berhasil",
												"📄 Laporan berhasil dibuat!\n\n"+outputDocx+"\n\nBuka file sekarang?",
												func(open bool) {
													if open {
														openFile(outputDocx)
													}
												}, w)
										})
									}()
								}

								// Laporan Harian wajib melalui dialog data tiket
								if !cfg.IsMonthly {
									fyne.Do(func() { pickTicketData(w, doGenerate) })
								} else {
									doGenerate("")
								}
							})
						}

						successDlg.Show()
					}
				}
			}()
		}

		d := dialog.NewCustomConfirm("▶  Mulai Capture", "▶  Mulai", "Batal", modeContent,
			func(ok bool) {
				if ok {
					launchCapture()
				}
			}, w)
		d.Resize(fyne.NewSize(500, 400))
		d.Show()
	}

	// ── Main Layout ───────────────────────────────────────────────────────────
	mainContent := container.NewVScroll(
		container.NewPadded(
			container.NewVBox(
				container.NewPadded(mainHeader),
				container.NewGridWithColumns(2, loginCard, captureCard),
				container.NewGridWithColumns(2, dateCard, outputCard),
				container.NewPadded(bottomRow),
				container.NewPadded(logSection),
			),
		),
	)

	// ── Konfirmasi tutup saat capture sedang berjalan ─────────────────────────
	w.SetCloseIntercept(func() {
		if isCapturing {
			dialog.ShowConfirm(
				"Tutup Aplikasi",
				"Jika aplikasi ditutup, semua aktivitas capture\nyang sedang berjalan akan terhenti.\n\nApakah Anda yakin ingin menutup aplikasi?",
				func(ok bool) {
					if ok {
						if currentCancel != nil {
							currentCancel()
						}
						w.Close()
					}
				}, w)
		} else {
			w.Close()
		}
	})

	root := container.New(&sidebarLayout{w: 210}, sidebar, mainContent)
	return root
}
