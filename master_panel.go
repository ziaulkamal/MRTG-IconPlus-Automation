package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"io"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ── Server-side user record (mirrors server JSON) ────────────────────────────

type TrackedUser struct {
	MAC        string    `json:"mac"`
	IP         string    `json:"ip"`
	City       string    `json:"city"`
	Country    string    `json:"country"`
	ISP        string    `json:"isp"`
	Lat        float64   `json:"lat"`
	Lon        float64   `json:"lon"`
	Version    string    `json:"version"`
	LastSeen   time.Time `json:"last_seen"`
	Blocked    bool      `json:"blocked"`
}

// ── Master panel entry point ─────────────────────────────────────────────────

// showMasterPanelPrompt shows a password dialog; on success opens the panel.
func showMasterPanelPrompt(w fyne.Window) {
	pwEntry := widget.NewPasswordEntry()
	pwEntry.SetPlaceHolder("Password master")

	var dlg dialog.Dialog
	okBtn := widget.NewButton("Masuk", func() {
		if pwEntry.Text == trackingMasterPwd {
			dlg.Hide()
			openMasterPanel(w)
		} else {
			dialog.ShowError(fmt.Errorf("password salah"), w)
		}
	})
	okBtn.Importance = widget.HighImportance

	pwEntry.OnSubmitted = func(_ string) { okBtn.OnTapped() }

	content := container.NewVBox(
		widget.NewLabel("Masukkan password untuk membuka Master Panel:"),
		pwEntry,
		container.NewCenter(okBtn),
	)

	dlg = dialog.NewCustom("🔐 Master Panel", "Batal", content, w)
	dlg.Resize(fyne.NewSize(360, 180))
	dlg.Show()
}

// ── Master panel window ──────────────────────────────────────────────────────

func openMasterPanel(w fyne.Window) {
	users, err := fetchUsers()
	if err != nil {
		dialog.ShowError(fmt.Errorf("Gagal mengambil data pengguna:\n%v", err), w)
		return
	}

	refreshBtn := widget.NewButton("🔄 Refresh", nil)
	titleTxt := canvas.NewText("👥 Master Panel — Pengguna Aktif", colNavy)
	titleTxt.TextStyle = fyne.TextStyle{Bold: true}
	titleTxt.TextSize = 15

	var rows []fyne.CanvasObject

	buildRows := func(users []TrackedUser) []fyne.CanvasObject {
		var result []fyne.CanvasObject
		for _, u := range users {
			u := u

			statusDot := "🟢"
			if u.Blocked {
				statusDot = "🔴"
			}

			lastSeenStr := u.LastSeen.Format("02/01 15:04")

			macTxt := canvas.NewText(statusDot+" "+u.MAC, colNavy)
			macTxt.TextStyle = fyne.TextStyle{Bold: true}
			macTxt.TextSize = 12

			ipTxt := canvas.NewText("IP: "+u.IP, colTxtGray)
			ipTxt.TextSize = 11

			locTxt := canvas.NewText(
				fmt.Sprintf("📍 %s, %s  |  ISP: %s", u.City, u.Country, u.ISP),
				colTxtGray,
			)
			locTxt.TextSize = 11

			gpsTxt := canvas.NewText(
				fmt.Sprintf("GPS: %.4f, %.4f", u.Lat, u.Lon),
				color.NRGBA{R: 100, G: 140, B: 100, A: 255},
			)
			gpsTxt.TextSize = 10

			seenTxt := canvas.NewText("Terakhir: "+lastSeenStr+"  |  v"+u.Version, colTxtGray)
			seenTxt.TextSize = 10

			var toggleBtn *widget.Button
			if u.Blocked {
				toggleBtn = widget.NewButton("✅ Unblock", func() {
					if err := sendBlockAction(u.MAC, false); err != nil {
						fyne.Do(func() { dialog.ShowError(err, w) })
					} else {
						fyne.Do(func() {
							dialog.ShowInformation("Berhasil", "Perangkat berhasil di-unblock.", w)
						})
					}
				})
				toggleBtn.Importance = widget.SuccessImportance
			} else {
				toggleBtn = widget.NewButton("🚫 Block", func() {
					dialog.ShowConfirm(
						"Konfirmasi Block",
						"Apakah Anda yakin ingin memblokir perangkat ini?\nMAC: "+u.MAC,
						func(yes bool) {
							if !yes {
								return
							}
							if err := sendBlockAction(u.MAC, true); err != nil {
								dialog.ShowError(err, w)
							} else {
								dialog.ShowInformation("Berhasil", "Perangkat berhasil diblokir.", w)
							}
						}, w)
				})
				toggleBtn.Importance = widget.DangerImportance
			}

			infoCol := container.NewVBox(macTxt, ipTxt, locTxt, gpsTxt, seenTxt)
			row := container.NewBorder(nil, nil, nil, container.NewCenter(toggleBtn), infoCol)

			bg := canvas.NewRectangle(colWhite)
			bg.CornerRadius = 8
			bg.StrokeColor = colBorder
			bg.StrokeWidth = 1

			card := container.NewStack(bg, container.NewPadded(container.NewPadded(row)))
			result = append(result, card)
		}
		return result
	}

	rows = buildRows(users)

	listBox := container.NewVBox(rows...)
	scroll := container.NewVScroll(container.NewPadded(listBox))
	scroll.SetMinSize(fyne.NewSize(620, 400))

	countTxt := canvas.NewText(
		fmt.Sprintf("Total terdaftar: %d  |  Data dari: %s", len(users), trackingServerURL),
		colTxtGray,
	)
	countTxt.TextSize = 10

	sep := canvas.NewRectangle(colBorder)
	sep.SetMinSize(fyne.NewSize(0, 1))

	header := container.NewBorder(nil, nil, titleTxt, refreshBtn, nil)

	content := container.NewBorder(
		container.NewVBox(header, sep),
		container.NewPadded(countTxt),
		nil, nil,
		scroll,
	)

	var panelDlg dialog.Dialog

	refreshBtn.OnTapped = func() {
		refreshBtn.Disable()
		go func() {
			newUsers, err := fetchUsers()
			fyne.Do(func() {
				refreshBtn.Enable()
				if err != nil {
					dialog.ShowError(fmt.Errorf("Refresh gagal: %v", err), w)
					return
				}
				newRows := buildRows(newUsers)
				listBox.Objects = newRows
				listBox.Refresh()
				countTxt.Text = fmt.Sprintf(
					"Total terdaftar: %d  |  Data dari: %s", len(newUsers), trackingServerURL)
				countTxt.Refresh()
			})
		}()
	}

	panelDlg = dialog.NewCustom("🔐 Master Panel", "Tutup", content, w)
	panelDlg.Resize(fyne.NewSize(700, 560))
	panelDlg.Show()
}

// ── API calls ────────────────────────────────────────────────────────────────

func fetchUsers() ([]TrackedUser, error) {
	url := fmt.Sprintf("%s/api/users?key=%s", trackingServerURL, trackingMasterPwd)
	resp, err := telemetryClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var users []TrackedUser
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}
	return users, nil
}

type blockRequest struct {
	Key  string `json:"key"`
	MAC  string `json:"mac"`
}

func sendBlockAction(mac string, block bool) error {
	endpoint := "/api/block"
	if !block {
		endpoint = "/api/unblock"
	}
	return postJSON(
		trackingServerURL+endpoint,
		blockRequest{Key: trackingMasterPwd, MAC: mac},
		nil,
	)
}

// fetchActiveCount returns just the active count (for sidebar polling).
func fetchActiveCount() int {
	var cr countResponse
	if err := getJSON(trackingServerURL+"/api/count", &cr); err != nil {
		return -1
	}
	return cr.Active
}

