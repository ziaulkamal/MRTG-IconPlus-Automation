package main

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

var bulanPanjang = [...]string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

var bulanPendek = [...]string{
	"Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
	"Jul", "Agu", "Sep", "Okt", "Nov", "Des",
}

var hariHeader = [...]string{"Sen", "Sel", "Rab", "Kam", "Jum", "Sab", "Min"}

// showDatePicker menampilkan kalender untuk memilih tanggal penuh (YYYY/MM/DD).
// onSelect dipanggil dengan tanggal terpilih saat user menekan "Pilih".
func showDatePicker(initial time.Time, onSelect func(time.Time), parent fyne.Window) {
	if initial.IsZero() {
		initial = time.Now()
	}

	// cur = bulan yang sedang ditampilkan, sel = tanggal yang dipilih
	cur := time.Date(initial.Year(), initial.Month(), 1, 0, 0, 0, 0, time.Local)
	sel := initial

	navLbl := canvas.NewText("", colTxtDark)
	navLbl.TextStyle = fyne.TextStyle{Bold: true}
	navLbl.TextSize = 14
	navLbl.Alignment = fyne.TextAlignCenter

	grid := container.NewGridWithColumns(7)

	var rebuild func()
	rebuild = func() {
		navLbl.Text = fmt.Sprintf("%s  %d", bulanPanjang[cur.Month()-1], cur.Year())
		canvas.Refresh(navLbl)

		cells := make([]fyne.CanvasObject, 0, 49)

		// Header hari
		for _, d := range hariHeader {
			hdr := canvas.NewText(d, colTxtGray)
			hdr.TextSize = 11
			hdr.Alignment = fyne.TextAlignCenter
			cells = append(cells, container.NewCenter(hdr))
		}

		// Offset hari pertama (Senin = 0, Minggu = 6)
		firstDay := time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, time.Local)
		offset := int(firstDay.Weekday()) - 1
		if offset < 0 {
			offset = 6 // Minggu
		}
		for i := 0; i < offset; i++ {
			cells = append(cells, widget.NewLabel(""))
		}

		// Tombol hari
		daysInMonth := time.Date(cur.Year(), cur.Month()+1, 0, 0, 0, 0, 0, time.Local).Day()
		curSnap := cur
		for d := 1; d <= daysInMonth; d++ {
			day := d
			isSelected := sel.Year() == curSnap.Year() &&
				sel.Month() == curSnap.Month() &&
				sel.Day() == day

			btn := widget.NewButton(fmt.Sprintf("%d", day), func() {
				sel = time.Date(curSnap.Year(), curSnap.Month(), day, 0, 0, 0, 0, time.Local)
				rebuild()
			})
			if isSelected {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = widget.LowImportance
			}
			cells = append(cells, btn)
		}

		grid.Objects = cells
		grid.Refresh()
	}

	prevBtn := widget.NewButton("◀", func() {
		cur = cur.AddDate(0, -1, 0)
		rebuild()
	})
	prevBtn.Importance = widget.LowImportance

	nextBtn := widget.NewButton("▶", func() {
		cur = cur.AddDate(0, 1, 0)
		rebuild()
	})
	nextBtn.Importance = widget.LowImportance

	sep := canvas.NewRectangle(colBorder)
	sep.SetMinSize(fyne.NewSize(0, 1))

	nav := container.NewBorder(nil, nil, prevBtn, nextBtn, container.NewCenter(navLbl))
	content := container.NewVBox(nav, sep, grid)

	rebuild()

	d := dialog.NewCustomConfirm("Pilih Tanggal", "✔  Pilih", "Batal", content,
		func(ok bool) {
			if ok {
				onSelect(sel)
			}
		}, parent)
	d.Resize(fyne.NewSize(320, 370))
	d.Show()
}

// showMonthPicker menampilkan pemilih tahun + bulan (untuk format YYYY/MM).
// onSelect dipanggil dengan tanggal terpilih (hari selalu 1) saat user menekan "Pilih".
func showMonthPicker(initial time.Time, onSelect func(time.Time), parent fyne.Window) {
	if initial.IsZero() {
		initial = time.Now()
	}

	selYear := initial.Year()
	selMonth := initial.Month()

	yearLbl := canvas.NewText(fmt.Sprintf("%d", selYear), colTxtDark)
	yearLbl.TextStyle = fyne.TextStyle{Bold: true}
	yearLbl.TextSize = 16
	yearLbl.Alignment = fyne.TextAlignCenter

	grid := container.NewGridWithColumns(3)

	var rebuild func()
	rebuild = func() {
		yearLbl.Text = fmt.Sprintf("%d", selYear)
		canvas.Refresh(yearLbl)

		cells := make([]fyne.CanvasObject, 12)
		for i := 0; i < 12; i++ {
			m := time.Month(i + 1)
			btn := widget.NewButton(bulanPendek[i], func() {
				selMonth = m
				rebuild()
			})
			if m == selMonth {
				btn.Importance = widget.HighImportance
			} else {
				btn.Importance = widget.LowImportance
			}
			cells[i] = btn
		}
		grid.Objects = cells
		grid.Refresh()
	}

	prevYear := widget.NewButton("◀", func() { selYear--; rebuild() })
	prevYear.Importance = widget.LowImportance
	nextYear := widget.NewButton("▶", func() { selYear++; rebuild() })
	nextYear.Importance = widget.LowImportance

	sep := canvas.NewRectangle(colBorder)
	sep.SetMinSize(fyne.NewSize(0, 1))

	nav := container.NewBorder(nil, nil, prevYear, nextYear, container.NewCenter(yearLbl))
	content := container.NewVBox(nav, sep, grid)

	rebuild()

	d := dialog.NewCustomConfirm("Pilih Bulan", "✔  Pilih", "Batal", content,
		func(ok bool) {
			if ok {
				onSelect(time.Date(selYear, selMonth, 1, 0, 0, 0, 0, time.Local))
			}
		}, parent)
	d.Resize(fyne.NewSize(290, 260))
	d.Show()
}
