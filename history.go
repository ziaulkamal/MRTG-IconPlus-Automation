package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// HistoryEntry menyimpan satu sesi capture.
type HistoryEntry struct {
	Timestamp    string `json:"timestamp"`     // "21 Mei 2026, 14:30:05"
	OutputFolder string `json:"output_folder"` // path folder output
	SIDCount     int    `json:"sid_count"`
	DateRange    string `json:"date_range"`   // "01/05/2026 – 31/05/2026"
	TotalFiles   int    `json:"total_files"`
	HasError     bool   `json:"has_error"`
	Cancelled    bool   `json:"cancelled"` // true jika dibatalkan pengguna
}

// historyFilePath mengembalikan path file riwayat di %TEMP% (atau /tmp).
func historyFilePath() string {
	return filepath.Join(os.TempDir(), "mrtg_capture_history.json")
}

// loadHistory membaca riwayat dari file. Mengembalikan slice kosong jika file belum ada.
func loadHistory() []HistoryEntry {
	data, err := os.ReadFile(historyFilePath())
	if err != nil {
		return nil
	}
	var entries []HistoryEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil
	}
	return entries
}

// appendHistoryEntry menambahkan entri baru (terbaru di atas), maks 100 entri.
func appendHistoryEntry(entry HistoryEntry) {
	entries := loadHistory()
	entries = append([]HistoryEntry{entry}, entries...) // newest first
	if len(entries) > 100 {
		entries = entries[:100]
	}
	data, _ := json.MarshalIndent(entries, "", "  ")
	os.WriteFile(historyFilePath(), data, 0644)
}

// showHistoryDialog menampilkan dialog riwayat capture.
func showHistoryDialog(w fyne.Window) {
	entries := loadHistory()

	if len(entries) == 0 {
		dialog.ShowInformation("Riwayat Capture",
			"Belum ada riwayat capture.\nJalankan capture terlebih dahulu.", w)
		return
	}

	var cards []fyne.CanvasObject
	for _, e := range entries {
		entry := e

		// Warna dan ikon status
		statusColor := colLogOk
		statusIcon := "✅"
		statusText := "Selesai"
		switch {
		case entry.Cancelled:
			statusColor = colTxtGray
			statusIcon = "⛔"
			statusText = "Dibatalkan"
		case entry.HasError:
			statusColor = color.NRGBA{R: 255, G: 165, B: 0, A: 255}
			statusIcon = "⚠"
			statusText = "Selesai dengan error"
		}

		tsTxt := canvas.NewText("🕐  "+entry.Timestamp, colTxtGray)
		tsTxt.TextSize = 11

		folderTxt := canvas.NewText("📁  "+entry.OutputFolder, colNavy)
		folderTxt.TextStyle = fyne.TextStyle{Bold: true}
		folderTxt.TextSize = 12

		detailTxt := canvas.NewText(
			fmt.Sprintf("📊  %d SID  •  %s  •  %d file",
				entry.SIDCount, entry.DateRange, entry.TotalFiles),
			colTxtGray,
		)
		detailTxt.TextSize = 11

		statusTxt := canvas.NewText(statusIcon+"  "+statusText, statusColor)
		statusTxt.TextSize = 11

		openBtn := widget.NewButton("📂  Buka Folder", func() {
			openOutputFolder(entry.OutputFolder)
		})
		openBtn.Importance = widget.LowImportance

		infoCol := container.NewVBox(tsTxt, folderTxt, detailTxt, statusTxt)
		row := container.NewBorder(nil, nil, nil, container.NewCenter(openBtn), infoCol)

		bg := canvas.NewRectangle(colWhite)
		bg.CornerRadius = 10
		bg.StrokeColor = colBorder
		bg.StrokeWidth = 1.2

		card := container.NewStack(bg, container.NewPadded(container.NewPadded(row)))
		cards = append(cards, card)
	}

	listBox := container.NewVBox(cards...)
	scroll := container.NewVScroll(container.NewPadded(listBox))
	scroll.SetMinSize(fyne.NewSize(560, 400))

	// Footer: lokasi file riwayat
	filePath := historyFilePath()
	footerTxt := canvas.NewText("💾  Riwayat tersimpan di: "+filePath, colTxtGray)
	footerTxt.TextSize = 10

	countTxt := canvas.NewText(
		fmt.Sprintf("%d riwayat capture tercatat", len(entries)),
		colTxtGray,
	)
	countTxt.TextSize = 11

	sep := canvas.NewRectangle(colBorder)
	sep.SetMinSize(fyne.NewSize(0, 1))

	content := container.NewBorder(
		container.NewVBox(container.NewPadded(countTxt), sep),
		container.NewPadded(footerTxt),
		nil, nil,
		scroll,
	)

	d := dialog.NewCustom("🕐  Riwayat Capture", "Tutup", content, w)
	d.Resize(fyne.NewSize(620, 560))
	d.Show()
}

// nowTimestamp mengembalikan timestamp format Indonesia untuk disimpan ke riwayat.
func nowTimestamp() string {
	t := time.Now()
	return fmt.Sprintf("%02d %s %d, %02d:%02d:%02d",
		t.Day(), bulanPanjang[t.Month()-1], t.Year(),
		t.Hour(), t.Minute(), t.Second())
}
