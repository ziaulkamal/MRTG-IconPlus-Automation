# MRTG Chart Automation

**Automasi capture grafik MRTG untuk monitoring jaringan PLN Icon Plus.**

> Developed by **ZIAUL KAMAL** — EOS Aceh Barat Daya

---

## Fitur Utama

| Fitur | Keterangan |
|---|---|
| 🔐 Login fleksibel | Via SID atau username/password |
| 🎯 Single Capture | Satu SID target |
| 📋 Multiple Capture | Banyak SID dari file `.txt` |
| 📅 Date Picker | Kalender interaktif (Full Month / Custom Range) |
| 💾 Auto-save | Output `[SID]_[DD-MM-YYYY].jpg` |
| 🕐 Riwayat | Log capture tersimpan otomatis di `%TEMP%` |
| 🔌 Cek Koneksi | Splash screen + verifikasi ke server MRTG saat startup |
| 📂 Buka Folder | Shortcut buka hasil setelah capture selesai |
| 🪟 Native Dialog | File & folder picker Windows Explorer asli |

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

# 2. Generate ikon exe (hanya sekali, atau jika pln-logo.png berubah)
go run gen_icon.go

# 3. Build
build.bat
```

Output: `mrtg_automation.exe`

> **Catatan:** Build pertama bisa memakan waktu 5–15 menit (Fyne CGo compile).
> Build berikutnya jauh lebih cepat (~10–30 detik).

---

## Cara Pakai

1. Jalankan `mrtg_automation.exe`
2. Aplikasi akan cek koneksi ke `mrtg.iconpln.co.id`
3. Isi form sesuai kebutuhan:

```
┌──────────────────────────────────────────────────────────┐
│  1. Login      → SID atau Username/Password              │
│  2. Capture    → Single (1 SID) atau Multiple (file)     │
│  3. Date Range → Full Month (YYYY/MM) atau Custom Range  │
│  4. Output     → Pilih folder penyimpanan                │
│                                                          │
│  ▶  Mulai Capture                                        │
└──────────────────────────────────────────────────────────┘
```

### Format File SID (`sid_list.txt`)

```
# Komentar diawali tanda pagar
231202003123
231202003130
231303001276
# dst...
```

### Nama File Output

```
231202003123_01-06-2026.jpg
231202003123_02-06-2026.jpg
...
```

---

## Struktur Proyek

```
mrtg/
├── mrtg_automation.go      # Core automation + CLI entry point
├── mrtg_gui.go             # GUI layout utama (Fyne)
├── splash.go               # Splash screen + cek koneksi
├── about.go                # Dialog "Tentang" + "Buy Me a Coffee"
├── datepicker.go           # Kalender interaktif (month & day picker)
├── history.go              # Riwayat capture (baca/tulis JSON)
├── filepicker_windows.go   # Native Windows file/folder dialog
├── filepicker_other.go     # Fallback Fyne dialog (non-Windows)
├── platform_windows.go     # Maximize window on Windows
├── platform_other.go       # Stub untuk non-Windows
├── gen_icon.go             # Generator ikon exe (jalankan sekali)
├── build.bat               # Script build Windows
├── go.mod / go.sum         # Go module
├── pln-logo.png            # Logo PLN (ikon exe & sidebar)
├── icon-pln-icon-plus.png  # Logo PLN Icon Plus (header aplikasi)
├── buyme-a-coffee.jpeg     # QR donasi
└── sid_list.txt            # Daftar SID (contoh / opsional)
```

---

## Riwayat Capture

Setiap sesi capture otomatis dicatat di:

```
%TEMP%\mrtg_capture_history.json
```

Berisi: waktu proses, folder output, jumlah SID, rentang tanggal, total file.
Bisa dilihat lewat **🕐 Riwayat Capture** di sidebar.

---

## Lisensi

Proyek ini dibuat untuk keperluan internal **PLN Icon Plus — EOS Aceh Barat Daya**.

---

## Donasi / Support

Jika proyek ini membantu pekerjaan Anda, pertimbangkan untuk berdonasi:

**ZIAUL KAMAL** — Bank Aceh · `09502200034703`

*(QR tersedia di tombol ☕ Buy Me a Coffee di dalam aplikasi)*
