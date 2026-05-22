//go:build windows

package main

import (
	"syscall"
	"time"
)

// maximizeOnStart memaksimalkan window setelah ditampilkan
func maximizeOnStart() {
	go func() {
		time.Sleep(250 * time.Millisecond)
		user32 := syscall.NewLazyDLL("user32.dll")
		getFW := user32.NewProc("GetForegroundWindow")
		showW := user32.NewProc("ShowWindow")
		hwnd, _, _ := getFW.Call()
		if hwnd != 0 {
			showW.Call(hwnd, 3) // SW_MAXIMIZE = 3
		}
	}()
}
