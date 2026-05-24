package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// HistoryEntry menyimpan satu sesi capture.
type HistoryEntry struct {
	Timestamp    string   `json:"timestamp"`     // "21 Mei 2026, 14:30:05"
	OutputFolder string   `json:"output_folder"` // path folder output
	SIDCount     int      `json:"sid_count"`
	DateRange    string   `json:"date_range"` // "01/05/2026 – 31/05/2026"
	TotalFiles   int      `json:"total_files"`
	HasError     bool     `json:"has_error"`
	Cancelled    bool     `json:"cancelled"`  // true jika dibatalkan pengguna
	IsMonthly    bool     `json:"is_monthly"` // true = Laporan Bulanan
	SIDs         []string `json:"sids"`       // daftar SID dalam urutan capture
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

// pickTicketData menampilkan dialog input data tiket mentah untuk Laporan Harian.
// Callback dipanggil dengan string data tiket (kosong jika tidak ada tiket).
func pickTicketData(w fyne.Window, callback func(ticketData string)) {
	entry := widget.NewMultiLineEntry()
	entry.SetPlaceHolder(
		"Paste data tiket di sini — mendukung 2 format:\n\n" +
			"① FORMAT CSV (direkomendasikan):\n" +
			"TICKET_ID,SID,TIPE,TGL_BUKA,JAM_BUKA,DURASI,DESKRIPSI,TGL_TUTUP,JAM_TUTUP,NAMA_CUSTOMER\n" +
			"RWR2C3N9,231202002130,METRONET,2026-04-14,10:12,4.830,PENYAMBUNGAN KABEL,2026-04-17,14:25,SEKRETARIAT MPU\n\n" +
			"② FORMAT RAW TEXT (spasi sebagai pemisah):\n" +
			"RWR2C3N9 231202002130 METRONET 2026-04-14 10:12 4.830 PENYAMBUNGAN KABEL 2026-04-17 14:25 SEKRETARIAT MPU\n\n" +
			"Format terdeteksi otomatis. Tiket open → tanggal buka • Tiket close → tanggal tutup.")

	scroll := container.NewVScroll(entry)
	scroll.SetMinSize(fyne.NewSize(680, 220))

	noteTxt := canvas.NewText(
		"💡  Tiket open → tanggal buka  •  Tiket close → tanggal tutup  •  Kosongkan jika tidak ada tiket",
		colTxtGray)
	noteTxt.TextSize = 10

	var dlg dialog.Dialog

	skipBtn := widget.NewButton("Tanpa Data Tiket", func() {
		dlg.Hide()
		callback("")
	})
	skipBtn.Importance = widget.LowImportance

	genBtn := widget.NewButton("✅  Generate Laporan", func() {
		dlg.Hide()
		callback(entry.Text)
	})
	genBtn.Importance = widget.HighImportance

	btnRow := container.NewBorder(nil, nil, skipBtn, genBtn, nil)
	content := container.NewVBox(
		scroll,
		container.NewPadded(noteTxt),
		container.NewPadded(btnRow),
	)

	dlg = dialog.NewCustom("📋  Data Tiket Laporan Harian", "Batal", content, w)
	dlg.Resize(fyne.NewSize(740, 380))
	dlg.Show()
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

		modeTxt := "Harian"
		if entry.IsMonthly {
			modeTxt = "Bulanan"
		}

		tsTxt := canvas.NewText("🕐  "+entry.Timestamp, colTxtGray)
		tsTxt.TextSize = 11

		folderTxt := canvas.NewText("📁  "+entry.OutputFolder, colNavy)
		folderTxt.TextStyle = fyne.TextStyle{Bold: true}
		folderTxt.TextSize = 12

		detailTxt := canvas.NewText(
			fmt.Sprintf("📊  %d SID  •  %s  •  %d file  •  Laporan %s",
				entry.SIDCount, entry.DateRange, entry.TotalFiles, modeTxt),
			colTxtGray,
		)
		detailTxt.TextSize = 11

		statusTxt := canvas.NewText(statusIcon+"  "+statusText, statusColor)
		statusTxt.TextSize = 11

		openBtn := widget.NewButton("📂  Folder", func() {
			openOutputFolder(entry.OutputFolder)
		})
		openBtn.Importance = widget.LowImportance

		// Tombol generate laporan DOCX
		laporanBtn := widget.NewButton("📄  Laporan", nil)
		laporanBtn.Importance = widget.MediumImportance
		if entry.Cancelled || len(entry.SIDs) == 0 {
			laporanBtn.Disable()
		}
		laporanBtn.OnTapped = func() {
			laporanBtn.Disable()
			pickDocx("Pilih Template DOCX ("+modeTxt+")", w, func(templatePath string) {
				if templatePath == "" {
					fyne.Do(func() { laporanBtn.Enable() })
					return
				}

				// Parse tanggal dari DateRange "01/05/2026 – 31/05/2026"
				var startDate, endDate time.Time
				parts := strings.SplitN(entry.DateRange, " – ", 2)
				if len(parts) == 2 {
					startDate, _ = time.Parse("02/01/2006", strings.TrimSpace(parts[0]))
					endDate, _ = time.Parse("02/01/2006", strings.TrimSpace(parts[1]))
				}

				doGenerate := func(ticketData string) {
					var reportName string
					if entry.IsMonthly {
						reportName = fmt.Sprintf("Laporan_Bulanan_%s.docx", startDate.Format("01-2006"))
					} else {
						reportName = fmt.Sprintf("Laporan_Harian_%s_%s.docx",
							startDate.Format("02-01-2006"), endDate.Format("02-01-2006"))
					}
					outputDocx := filepath.Join(entry.OutputFolder, reportName)

					rcfg := &ReportGenConfig{
						TemplatePath: templatePath,
						OutputPath:   outputDocx,
						IsMonthly:    entry.IsMonthly,
						SIDs:         entry.SIDs,
						ImageDir:     entry.OutputFolder,
						StartDate:    startDate,
						EndDate:      endDate,
						TicketData:   ticketData,
					}

					fyne.Do(func() { laporanBtn.SetText("⏳") })
					go func() {
						defer fyne.Do(func() {
							laporanBtn.SetText("📄  Laporan")
							laporanBtn.Enable()
						})
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
				if !entry.IsMonthly {
					fyne.Do(func() { pickTicketData(w, doGenerate) })
				} else {
					doGenerate("")
				}
			})
		}

		infoCol := container.NewVBox(tsTxt, folderTxt, detailTxt, statusTxt)
		btnCol := container.NewVBox(openBtn, laporanBtn)
		row := container.NewBorder(nil, nil, nil, container.NewCenter(btnCol), infoCol)

		bg := canvas.NewRectangle(colWhite)
		bg.CornerRadius = 10
		bg.StrokeColor = colBorder
		bg.StrokeWidth = 1.2

		card := container.NewStack(bg, container.NewPadded(container.NewPadded(row)))
		cards = append(cards, card)
	}

	listBox := container.NewVBox(cards...)
	scroll := container.NewVScroll(container.NewPadded(listBox))
	scroll.SetMinSize(fyne.NewSize(600, 400))

	// Footer
	filePath := historyFilePath()
	footerTxt := canvas.NewText("💾  Riwayat tersimpan di: "+filePath, colTxtGray)
	footerTxt.TextSize = 10

	countTxt := canvas.NewText(
		fmt.Sprintf("%d riwayat capture tercatat", len(entries)),
		colTxtGray,
	)
	countTxt.TextSize = 11

	noteTxt := canvas.NewText("💡 Tombol 📄 Laporan membutuhkan template DOCX dengan placeholder {{SID_CONTENT}}", colTxtGray)
	noteTxt.TextSize = 10

	sep := canvas.NewRectangle(colBorder)
	sep.SetMinSize(fyne.NewSize(0, 1))

	content := container.NewBorder(
		container.NewVBox(container.NewPadded(countTxt), sep),
		container.NewVBox(container.NewPadded(noteTxt), container.NewPadded(footerTxt)),
		nil, nil,
		scroll,
	)

	d := dialog.NewCustom("🕐  Riwayat Capture", "Tutup", content, w)
	d.Resize(fyne.NewSize(680, 580))
	d.Show()
}

// nowTimestamp mengembalikan timestamp format Indonesia untuk disimpan ke riwayat.
func nowTimestamp() string {
	t := time.Now()
	return fmt.Sprintf("%02d %s %d, %02d:%02d:%02d",
		t.Day(), bulanPanjang[t.Month()-1], t.Year(),
		t.Hour(), t.Minute(), t.Second())
}
