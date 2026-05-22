//go:build ignore

// Jalankan sekali: go run gen_icon.go
// Akan membuat app_windows.syso yang otomatis di-link saat go build
// sehingga ikon exe di Windows Explorer = pln-logo.png

package main

import (
	"encoding/binary"
	"log"
	"os"
	"os/exec"
)

func main() {
	png, err := os.ReadFile("pln-logo.png")
	if err != nil {
		log.Fatal("Gagal baca pln-logo.png:", err)
	}

	// Buat ICO yang membungkus data PNG (format PNG-in-ICO, Windows Vista+)
	ico, err := os.Create("pln-logo.ico")
	if err != nil {
		log.Fatal("Gagal buat ico:", err)
	}

	imageOffset := uint32(6 + 16)
	imageSize := uint32(len(png))

	// ICO header
	binary.Write(ico, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1)) // type = 1 (icon)
	binary.Write(ico, binary.LittleEndian, uint16(1)) // count = 1 image

	// Directory entry (16 bytes)
	binary.Write(ico, binary.LittleEndian, uint8(0))   // width  (0 = 256)
	binary.Write(ico, binary.LittleEndian, uint8(0))   // height (0 = 256)
	binary.Write(ico, binary.LittleEndian, uint8(0))   // color count
	binary.Write(ico, binary.LittleEndian, uint8(0))   // reserved
	binary.Write(ico, binary.LittleEndian, uint16(1))  // planes
	binary.Write(ico, binary.LittleEndian, uint16(32)) // bit count
	binary.Write(ico, binary.LittleEndian, imageSize)
	binary.Write(ico, binary.LittleEndian, imageOffset)

	ico.Write(png)
	ico.Close()
	log.Println("✅ Dibuat: pln-logo.ico")

	// Buat resource file
	rc := `IDI_ICON1 ICON "pln-logo.ico"` + "\n"
	if err := os.WriteFile("app.rc", []byte(rc), 0644); err != nil {
		log.Fatal("Gagal buat app.rc:", err)
	}

	// Compile dengan windres (MinGW) → .syso (otomatis di-link saat build)
	cmd := exec.Command("windres", "app.rc", "-O", "coff", "-o", "app_windows.syso")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal("windres gagal (pastikan MinGW ada di PATH):", err)
	}
	log.Println("✅ Dibuat: app_windows.syso")
	log.Println("Sekarang jalankan build.bat untuk membuat exe dengan ikon PLN")
}
