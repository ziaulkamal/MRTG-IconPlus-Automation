package main

import (
	"bytes"
	"image"
	"image/draw"
	"image/jpeg"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// qrResource memuat buyme-a-coffee.jpeg dan memotong area QR saja
// (membuang teks nama di atas dan logo bank di bawah).
func qrResource() fyne.Resource {
	data, err := embedFS.ReadFile("buyme-a-coffee.jpeg")
	if err != nil {
		return nil
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fyne.NewStaticResource("qr.jpg", data)
	}

	b := img.Bounds()
	// Potong: lewati ~17% atas (nama/nomor) dan ~18% bawah (logo bank)
	y0 := b.Min.Y + int(float64(b.Dy())*0.17)
	y1 := b.Min.Y + int(float64(b.Dy())*0.82)

	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), y1-y0))
	draw.Draw(dst, dst.Bounds(), img, image.Pt(b.Min.X, y0), draw.Src)

	var buf bytes.Buffer
	jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 92}) //nolint:errcheck
	return fyne.NewStaticResource("qr_cropped.jpg", buf.Bytes())
}

// showCoffeeDialog menampilkan popup QR donasi.
func showCoffeeDialog(w fyne.Window) {
	qrImg := canvas.NewImageFromResource(qrResource())
	qrImg.FillMode = canvas.ImageFillContain
	qrImg.SetMinSize(fyne.NewSize(280, 280))

	titleTxt := canvas.NewText("Terima kasih atas dukungannya!", colNavy)
	titleTxt.TextStyle = fyne.TextStyle{Bold: true}
	titleTxt.TextSize = 15
	titleTxt.Alignment = fyne.TextAlignCenter

	scanTxt := canvas.NewText("Scan QR untuk transfer via Bank Aceh", colTxtGray)
	scanTxt.TextSize = 12
	scanTxt.Alignment = fyne.TextAlignCenter

	sep := canvas.NewRectangle(colBorder)
	sep.SetMinSize(fyne.NewSize(200, 1))

	nameTxt := canvas.NewText("ZIAUL KAMAL", colNavy)
	nameTxt.TextStyle = fyne.TextStyle{Bold: true}
	nameTxt.TextSize = 15
	nameTxt.Alignment = fyne.TextAlignCenter

	accTxt := canvas.NewText("09502200034703", colTxtGray)
	accTxt.TextSize = 13
	accTxt.Alignment = fyne.TextAlignCenter

	content := container.NewVBox(
		container.NewCenter(titleTxt),
		container.NewCenter(scanTxt),
		container.NewPadded(container.NewCenter(qrImg)),
		container.NewCenter(sep),
		widget.NewLabel(""),
		container.NewCenter(nameTxt),
		container.NewCenter(accTxt),
	)

	d := dialog.NewCustom("☕  Buy Me a Coffee", "Tutup", content, w)
	d.Resize(fyne.NewSize(340, 490))
	d.Show()
}

// showAboutDialog menampilkan dialog dua kolom:
// kiri = informasi aplikasi, kanan = QR donasi.
func showAboutDialog(w fyne.Window) {
	// ── Kolom kiri: info aplikasi ─────────────────────────────────────────────
	appName := canvas.NewText("MRTG Chart Automation", colNavy)
	appName.TextStyle = fyne.TextStyle{Bold: true}
	appName.TextSize = 16

	version := canvas.NewText("Versi 1.0.0", colTxtGray)
	version.TextSize = 12

	descLabel := widget.NewLabel(
		"Automasi capture grafik MRTG untuk\n" +
			"monitoring jaringan PLN Icon Plus.\n" +
			"Output: [SID]_[DD-MM-YYYY].jpg",
	)
	descLabel.Wrapping = fyne.TextWrapWord

	sep1 := canvas.NewRectangle(colBorder)
	sep1.SetMinSize(fyne.NewSize(0, 1))

	devBy := canvas.NewText("Dikembangkan oleh:", colTxtGray)
	devBy.TextSize = 11

	devName := canvas.NewText("ZIAUL KAMAL", colNavy)
	devName.TextStyle = fyne.TextStyle{Bold: true}
	devName.TextSize = 14

	devUnit := canvas.NewText("EOS Aceh Barat Daya", colTxtGray)
	devUnit.TextSize = 11

	sep2 := canvas.NewRectangle(colBorder)
	sep2.SetMinSize(fyne.NewSize(0, 1))

	builtWith := canvas.NewText("Dibangun dengan:", colTxtGray)
	builtWith.TextSize = 11

	techStack := canvas.NewText("Go  ·  Fyne Framework  ·  chromedp", colTxtDark)
	techStack.TextSize = 11

	leftCol := container.NewVBox(
		appName,
		version,
		widget.NewLabel(""),
		descLabel,
		container.NewPadded(sep1),
		devBy,
		devName,
		devUnit,
		container.NewPadded(sep2),
		builtWith,
		techStack,
	)

	// ── Kolom kanan: QR donasi ────────────────────────────────────────────────
	qrImg := canvas.NewImageFromResource(qrResource())
	qrImg.FillMode = canvas.ImageFillContain
	qrImg.SetMinSize(fyne.NewSize(195, 195))

	coffeeTitle := canvas.NewText("☕  Buy Me a Coffee", colNavy)
	coffeeTitle.TextStyle = fyne.TextStyle{Bold: true}
	coffeeTitle.TextSize = 13
	coffeeTitle.Alignment = fyne.TextAlignCenter

	bankTxt := canvas.NewText("Transfer via Bank Aceh", colTxtGray)
	bankTxt.TextSize = 10
	bankTxt.Alignment = fyne.TextAlignCenter

	qrSep := canvas.NewRectangle(colBorder)
	qrSep.SetMinSize(fyne.NewSize(0, 1))

	qrName := canvas.NewText("ZIAUL KAMAL", colNavy)
	qrName.TextStyle = fyne.TextStyle{Bold: true}
	qrName.TextSize = 12
	qrName.Alignment = fyne.TextAlignCenter

	qrAcc := canvas.NewText("09502200034703", colTxtGray)
	qrAcc.TextSize = 11
	qrAcc.Alignment = fyne.TextAlignCenter

	rightCol := container.NewVBox(
		container.NewCenter(coffeeTitle),
		container.NewCenter(bankTxt),
		container.NewPadded(container.NewCenter(qrImg)),
		container.NewCenter(qrSep),
		widget.NewLabel(""),
		container.NewCenter(qrName),
		container.NewCenter(qrAcc),
	)

	// ── Divider vertikal antara dua kolom ─────────────────────────────────────
	vDiv := canvas.NewRectangle(colBorder)
	vDiv.SetMinSize(fyne.NewSize(1, 0))

	grid := container.NewGridWithColumns(3,
		container.NewPadded(leftCol),
		container.NewCenter(vDiv),
		container.NewPadded(rightCol),
	)

	d := dialog.NewCustom("ⓘ  Tentang Aplikasi", "Tutup", container.NewPadded(grid), w)
	d.Resize(fyne.NewSize(600, 400))
	d.Show()
}
