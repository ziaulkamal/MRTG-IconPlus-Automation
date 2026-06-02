# MRTG Chart Automation

**Automasi capture grafik MRTG + generate laporan DOCX untuk monitoring jaringan PLN Icon Plus.**

> Developed by **ZIAUL KAMAL** — EOS Aceh Barat Daya

---

## Fitur Utama

| Fitur | Keterangan |
|---|---|
| 🔐 Login fleksibel | Via kredensial internal |
| 🎯 Single / Multiple Capture | Satu ID atau banyak dari file `.txt` |
| 📅 Date Picker | Kalender interaktif — Full Month atau Custom Range |
| ⚡ Parallel Capture | Hingga 10 worker Chrome serentak |
| 🏷️ Deteksi Tipe Layanan | Otomatis mendeteksi tipe layanan dari judul chart |
| 📄 Generate Laporan DOCX | Dari template `.docx` dengan placeholder otomatis |
| 📊 Laporan Bulanan | 2 layanan per halaman, diurutkan berdasarkan tipe |
| 📰 Laporan Harian | Per hari: semua chart + Tiket Open/Close otomatis |
| 🎫 Parser Tiket | Mendukung format CSV dan raw text |
| 💾 Auto-save | Output gambar per ID per tanggal |
| 🕐 Riwayat Capture | Log sesi tersimpan lokal, bisa re-generate laporan kapan saja |
| 🔌 Cek Koneksi | Splash screen + verifikasi ke server saat startup |
| 🪟 Native Dialog | File & folder picker Windows Explorer asli |
| 👥 User Tracking | Jumlah pengguna aktif ditampilkan di sidebar |
| 🛡️ Master Panel | Admin tersembunyi: lihat MAC, IP lokasi, GPS, block/unblock perangkat dari jauh |

---

## Cara Build

### Prasyarat

1. **[Go 1.21+](https://go.dev/dl/)** — compiler Go
2. **[MSYS2 / MinGW-w64](https://www.msys2.org/)** — GCC untuk Fyne (CGo)
   ```
   pacman -S mingw-w64-x86_64-gcc
   ```
3. **[Google Chrome](https://www.google.com/chrome/)** — digunakan oleh chromedp untuk capture

### Langkah Build

```batch
# 1. Salin file konfigurasi
copy tracking_config.example.go tracking_config.go
# Edit tracking_config.go — isi URL server dan password

# 2. Unduh dependency
go mod tidy

# 3. Build (tanpa console window)
set CGO_ENABLED=1
set PATH=C:\msys64\mingw64\bin;%PATH%
go build -ldflags="-H windowsgui" -o mrtg_automation.exe .
```

Output: `mrtg_automation.exe`

> **Catatan:** Build pertama bisa memakan waktu 5–15 menit (Fyne CGo compile).
> Build berikutnya jauh lebih cepat (~10–30 detik).

---

## Cara Pakai

### 1. Capture Grafik

1. Jalankan `mrtg_automation.exe`
2. Aplikasi cek koneksi ke server MRTG
3. Isi form:

```
┌──────────────────────────────────────────────────────────┐
│  1. Login      → Kredensial internal                     │
│  2. Capture    → Single atau Multiple (dari file)        │
│  3. Date Range → Full Month (YYYY/MM) atau Custom Range  │
│  4. Output     → Pilih folder penyimpanan                │
│  5. Worker     → Normal (1) atau Multi-Process (2–10)    │
│                                                          │
│  ▶  Mulai Capture                                        │
└──────────────────────────────────────────────────────────┘
```

### 2. Generate Laporan

Setelah capture selesai **tanpa error**, tombol **📄 Generate Laporan** muncul otomatis.
Laporan juga bisa di-generate ulang kapan saja via **🕐 Riwayat Capture**.

#### Laporan Bulanan
1. Klik **📄 Generate Laporan**
2. Pilih file template `.docx` (Bulanan)
3. File laporan tersimpan di folder output capture

#### Laporan Harian
1. Klik **📄 Generate Laporan**
2. Pilih file template `.docx` (Harian)
3. **Wajib** input data tiket — paste dari sistem tiket (CSV atau raw text)
4. Klik **✅ Generate Laporan**

---

## Format File Daftar Layanan

File berisi daftar ID layanan yang akan di-capture — **satu ID per baris**.

```
# Komentar diawali tanda pagar
[ID_LAYANAN_1]
[ID_LAYANAN_2]
[ID_LAYANAN_3]
```

> ⚠️ File ini bersifat **sensitif** — jangan di-commit ke repository. Sudah terdaftar di `.gitignore`.

---

## Format Data Tiket

### Opsi 1 — CSV (direkomendasikan)

```csv
TICKET_ID,ID_LAYANAN,TIPE,TGL_BUKA,JAM_BUKA,DURASI,DESKRIPSI,TGL_TUTUP,JAM_TUTUP,NAMA_CUSTOMER
[TICKET_ID],[ID_LAYANAN],METRONET,2026-04-14,10:12,4.830,DESKRIPSI GANGGUAN,2026-04-17,14:25,NAMA CUSTOMER
```

### Opsi 2 — Raw Text (spasi sebagai pemisah)

```
[TICKET_ID] [ID_LAYANAN] METRONET 2026-04-14 10:12 4.830 DESKRIPSI GANGGUAN 2026-04-17 14:25 NAMA CUSTOMER
```

> Format terdeteksi **otomatis**. Tiket open → dikelompokkan di tanggal buka. Tiket close → di tanggal tutup.

---

## Template Laporan DOCX

Generate template sample dengan:

```batch
go run ./gen_template
```

Output: `Template_Laporan_Bulanan_SAMPLE.docx` dan `Template_Laporan_Harian_SAMPLE.docx`

### Placeholder yang didukung

| Placeholder | Laporan | Diganti dengan |
|---|---|---|
| `{{BULAN}}` | Bulanan & Harian | Nama bulan huruf besar — `MEI` |
| `{{TAHUN}}` | Bulanan & Harian | Tahun — `2026` |
| `{{SID_CONTENT}}` | **Wajib keduanya** | Konten chart + tiket (auto-generated) |

> `{{SID_CONTENT}}` harus berada di **paragraf sendiri** — satu baris, tidak boleh ada teks lain di baris yang sama.

---

## Struktur Proyek

```
mrtg/
├── mrtg_automation.go          # Core automation + worker pool + deteksi tipe layanan
├── mrtg_gui.go                 # GUI layout utama (Fyne)
├── report.go                   # Generate laporan DOCX (Bulanan & Harian)
├── splash.go                   # Splash screen + cek koneksi
├── history.go                  # Riwayat capture + dialog data tiket
├── datepicker.go               # Kalender interaktif
├── tracking_config.example.go  # Template konfigurasi tracking (salin & isi)
├── telemetry.go                # Client heartbeat + deteksi MAC + cek blocked
├── master_panel.go             # UI admin panel (user list, block/unblock)
├── filepicker_windows.go       # Native Windows file/folder dialog
├── filepicker_other.go         # Fallback Fyne dialog (non-Windows)
├── platform_windows.go         # Maximize window on Windows
├── platform_other.go           # Stub untuk non-Windows
├── gen_template/               # Utility generate sample template DOCX
├── server/                     # Tracking server (deploy ke VPS)
│   ├── main.go                 # HTTP server: heartbeat, count, users, block/unblock
│   └── go.mod
├── go.mod / go.sum             # Go module
├── pln-logo.png                # Logo PLN
├── icon-pln-icon-plus.png      # Logo PLN Icon Plus
└── buyme-a-coffee.jpeg         # QR donasi
```

> File-file berikut **tidak disertakan** di repository karena bersifat sensitif:
> `tracking_config.go` · `sid_list.txt` · `server/tracker_data.json`

---

## Riwayat Capture

Setiap sesi capture otomatis dicatat di:

```
%TEMP%\mrtg_capture_history.json
```

Berisi: waktu proses, folder output, jumlah layanan, rentang tanggal, tipe laporan.
Maksimal **100 entri** terakhir. Bisa dilihat & di-generate ulang via **🕐 Riwayat Capture** di sidebar.

---

## User Tracking & Master Panel

### Cara Kerja

Setiap kali aplikasi dibuka, klien mengirim **heartbeat** ke server tracking setiap **15 detik**.
Server mencatat MAC address, IP, lokasi (kota, negara, ISP, koordinat GPS via ip-api.com).

Pengguna dianggap **offline** jika tidak ada heartbeat selama **45 detik** (misalnya aplikasi ditutup).

Jumlah pengguna aktif ditampilkan di bagian bawah sidebar: **👥 N aktif**.

### Master Panel

Klik angka **👥** di sidebar → masukkan password master → panel terbuka.

Panel menampilkan:
- MAC address fisik perangkat
- Alamat IP dan lokasi (kota, negara, ISP)
- Koordinat GPS
- Waktu terakhir aktif dan versi aplikasi
- Tombol **🚫 Block** / **✅ Unblock** per perangkat

Perangkat yang diblokir akan mendapat dialog "Akses Diblokir" saat aplikasi dibuka.

### Deploy Server ke VPS

**Opsi 1 — Cross-compile dari Windows (direkomendasikan, tidak perlu Go di VPS):**

```powershell
# Build binary Linux dari PC Windows
cd server
$env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"
go build -o tracker-linux .

# Upload ke VPS
scp -P [PORT_SSH] tracker-linux user@IP_VPS:/opt/mrtg-tracker/tracker
```

**Opsi 2 — Build langsung di VPS:**

```bash
scp -r server/ user@vps:/opt/mrtg-tracker/
cd /opt/mrtg-tracker && go build -o tracker .
```

**Jalankan sebagai systemd service (auto-start saat reboot):**

```bash
# Buat file service
sudo nano /etc/systemd/system/mrtg-tracker.service
```

```ini
[Unit]
Description=MRTG Tracking Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/mrtg-tracker
ExecStart=/opt/mrtg-tracker/tracker
Environment=MASTER_KEY=passwordrahasia
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now mrtg-tracker
sudo ufw allow 24001/tcp
```

### Konfigurasi Klien

Salin `tracking_config.example.go` menjadi `tracking_config.go` lalu isi:

```go
const (
    trackingServerURL = "http://IP_VPS_ANDA:24001"
    trackingMasterPwd = "passwordrahasia"
    appVersion        = "1.1.0"
)
```

> **Catatan:** `tracking_config.go` tidak akan ter-push ke GitHub (sudah ada di `.gitignore`).

---

## Lisensi

Proyek ini dibuat untuk keperluan internal **PLN Icon Plus — EOS Aceh Barat Daya**.

---

*(QR donasi tersedia di tombol ☕ Buy Me a Coffee di dalam aplikasi)*
