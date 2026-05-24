# MRTG Chart Automation

**Automasi capture grafik MRTG + generate laporan DOCX untuk monitoring jaringan PLN Icon Plus.**

> Developed by **ZIAUL KAMAL** — EOS Aceh Barat Daya

---

## Fitur Utama

| Fitur | Keterangan |
|---|---|
| 🔐 Login fleksibel | Via SID atau username/password |
| 🎯 Single / Multiple Capture | Satu SID atau banyak SID dari file `.txt` |
| 📅 Date Picker | Kalender interaktif — Full Month atau Custom Range |
| ⚡ Parallel Capture | Hingga 10 worker Chrome serentak |
| 🏷️ Deteksi Tipe SID | Otomatis mendeteksi Backhaul / METRONET / INTERNET dari judul chart |
| 📄 Generate Laporan DOCX | Dari template `.docx` dengan placeholder otomatis |
| 📊 Laporan Bulanan | 2 SID per halaman, diurutkan: Backhaul → INTERNET → METRONET |
| 📰 Laporan Harian | Per hari: semua chart + Tiket Open/Close otomatis dari data tiket |
| 🎫 Parser Tiket | Mendukung format CSV dan raw text, distribusi per tanggal buka/tutup |
| 💾 Auto-save | Output `[SID]_[DD-MM-YYYY].jpg` atau `[SID]_[MM-YYYY].jpg` |
| 🕐 Riwayat Capture | Log sesi tersimpan di `%TEMP%`, bisa re-generate laporan kapan saja |
| 🔌 Cek Koneksi | Splash screen + verifikasi ke server MRTG saat startup |
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
# 1. Unduh dependency
go mod tidy

# 2. Build (tanpa console window)
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
2. Aplikasi cek koneksi ke `mrtg.iconpln.co.id`
3. Isi form:

```
┌──────────────────────────────────────────────────────────┐
│  1. Login      → SID atau Username/Password              │
│  2. Capture    → Single (1 SID) atau Multiple (file)     │
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

## Format File SID (`sid_list.txt`)

```
# Komentar diawali tanda pagar
231202003123
231202003130
231303001276
```

---

## Format Data Tiket

### Opsi 1 — CSV (direkomendasikan)

```csv
TICKET_ID,SID,TIPE,TGL_BUKA,JAM_BUKA,DURASI,DESKRIPSI,TGL_TUTUP,JAM_TUTUP,NAMA_CUSTOMER
RWR2C3N9,231202002130,METRONET,2026-04-14,10:12,4.830,PENYAMBUNGAN KABEL,2026-04-17,14:25,SEKRETARIAT MPU
EM26N5NF,231202002146,METRONET,2026-02-02,08:00,2.500,PERBAIKAN,2026-02-05,15:30,PKK
```

### Opsi 2 — Raw Text (spasi sebagai pemisah)

```
RWR2C3N9 231202002130 METRONET 2026-04-14 10:12 4.830 PENYAMBUNGAN KABEL 2026-04-17 14:25 SEKRETARIAT MPU
EM26N5NF 231202002146 METRONET 2026-02-02 08:00 2.500 PERBAIKAN 2026-02-05 15:30 PKK
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
| `{{TANGGAL_MULAI}}` | Harian | Tanggal awal — `01/02/2026` |
| `{{TANGGAL_AKHIR}}` | Harian | Tanggal akhir — `28/02/2026` |
| `{{SID_CONTENT}}` | **Wajib keduanya** | Konten chart + tiket (auto-generated) |

> `{{SID_CONTENT}}` harus berada di **paragraf sendiri** — satu baris, tidak boleh ada teks lain di baris yang sama.

---

## Nama File Output

```
# Laporan Harian
231202003123_01-02-2026.jpg
231202003123_02-02-2026.jpg

# Laporan Bulanan
231202003123_02-2026.jpg
```

---

## Struktur Proyek

```
mrtg/
├── mrtg_automation.go      # Core automation + worker pool + deteksi tipe SID
├── mrtg_gui.go             # GUI layout utama (Fyne)
├── report.go               # Generate laporan DOCX (Bulanan & Harian)
├── splash.go               # Splash screen + cek koneksi
├── history.go              # Riwayat capture + dialog data tiket
├── datepicker.go           # Kalender interaktif
├── tracking_config.go      # URL server tracking + password master + versi app
├── telemetry.go            # Client heartbeat + deteksi MAC + cek blocked
├── master_panel.go         # UI admin panel (user list, block/unblock)
├── filepicker_windows.go   # Native Windows file/folder dialog
├── filepicker_other.go     # Fallback Fyne dialog (non-Windows)
├── platform_windows.go     # Maximize window on Windows
├── platform_other.go       # Stub untuk non-Windows
├── gen_template/           # Utility generate sample template DOCX
├── server/                 # Tracking server (deploy ke VPS)
│   ├── main.go             # HTTP server: heartbeat, count, users, block/unblock
│   └── go.mod
├── go.mod / go.sum         # Go module
├── pln-logo.png            # Logo PLN
├── icon-pln-icon-plus.png  # Logo PLN Icon Plus
└── buyme-a-coffee.jpeg     # QR donasi
```

---

## Riwayat Capture

Setiap sesi capture otomatis dicatat di:

```
%TEMP%\mrtg_capture_history.json
```

Berisi: waktu proses, folder output, jumlah SID, rentang tanggal, tipe laporan, daftar SID.
Maksimal **100 entri** terakhir. Bisa dilihat & di-generate ulang via **🕐 Riwayat Capture** di sidebar.

---

## User Tracking & Master Panel

### Cara Kerja

Setiap kali aplikasi dibuka, klien mengirim **heartbeat** ke server tracking setiap 60 detik.
Server mencatat MAC address, IP, lokasi (kota, negara, ISP, koordinat GPS via ip-api.com).

Jumlah pengguna aktif ditampilkan di bagian bawah sidebar: **👥 N aktif**.

### Master Panel

Klik angka **👥** di sidebar → masukkan password master → panel terbuka.

Panel menampilkan:
- MAC address perangkat
- Alamat IP dan lokasi (kota, negara, ISP)
- Koordinat GPS (dari ip-api.com)
- Waktu terakhir aktif dan versi aplikasi
- Tombol **🚫 Block** / **✅ Unblock** per perangkat

Perangkat yang diblokir akan mendapat dialog "Akses Diblokir" saat aplikasi dibuka.

### Deploy Server ke VPS

```bash
# 1. Upload folder server/ ke VPS
scp -r server/ user@vps:/opt/mrtg-tracker/

# 2. Build di VPS
cd /opt/mrtg-tracker && go build -o tracker .

# 3. Jalankan (gunakan systemd atau screen)
MASTER_KEY=passwordrahasia ./tracker
```

Server berjalan di port **8787**.

### Konfigurasi Klien

Edit [tracking_config.go](tracking_config.go):

```go
const (
    trackingServerURL = "http://IP_VPS_ANDA:8787"
    trackingMasterPwd = "passwordrahasia"  // harus sama dengan MASTER_KEY
    appVersion        = "1.0.0"
)
```

Lalu build ulang aplikasi.

> **Catatan keamanan:** Jalankan server di balik reverse proxy (Nginx/Caddy) dengan HTTPS untuk produksi.

---

## Lisensi

Proyek ini dibuat untuk keperluan internal **PLN Icon Plus — EOS Aceh Barat Daya**.

---

*(QR donasi tersedia di tombol ☕ Buy Me a Coffee di dalam aplikasi)*
