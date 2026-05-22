//go:build windows

package main

import (
	"os/exec"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
)

var (
	comdlg32                = syscall.NewLazyDLL("comdlg32.dll")
	shell32fp               = syscall.NewLazyDLL("shell32.dll")
	ole32fp                 = syscall.NewLazyDLL("ole32.dll")
	procGetOpenFileNameW    = comdlg32.NewProc("GetOpenFileNameW")
	procSHBrowseForFolderW  = shell32fp.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32fp.NewProc("SHGetPathFromIDListW")
	procCoInitializeEx      = ole32fp.NewProc("CoInitializeEx")
	procCoUninitialize      = ole32fp.NewProc("CoUninitialize")
	procCoTaskMemFree       = ole32fp.NewProc("CoTaskMemFree")
)

// openFileNameW mirrors the Windows OPENFILENAMEW structure (x64: 152 bytes).
type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	_                 [4]byte // padding before next pointer
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	_                 [4]byte // padding before next pointer
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	flagsEx           uint32
}

// browseInfoW mirrors the Windows BROWSEINFOW structure.
type browseInfoW struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	_              [4]byte // padding before next pointer
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

// utf16Filter builds a double-null-terminated filter string for GetOpenFileNameW.
// pairs must alternate: description, pattern, description, pattern, ...
func utf16Filter(pairs ...string) []uint16 {
	var buf []uint16
	for _, s := range pairs {
		encoded, _ := syscall.UTF16FromString(s) // includes null terminator
		buf = append(buf, encoded...)
	}
	buf = append(buf, 0) // second null → double-null termination
	return buf
}

// pickFile shows the native Windows "Open File" dialog (Windows Explorer style).
// callback is called with the selected path; nothing happens if user cancels.
func pickFile(title string, _ fyne.Window, callback func(string)) {
	go func() {
		filter := utf16Filter(
			"File Teks (*.txt)", "*.txt",
			"Semua File (*.*)", "*.*",
		)
		titlePtr, _ := syscall.UTF16PtrFromString(title)
		fileBuf := make([]uint16, 32768)

		ofn := openFileNameW{
			lStructSize: uint32(unsafe.Sizeof(openFileNameW{})),
			lpstrFilter: &filter[0],
			lpstrTitle:  titlePtr,
			lpstrFile:   &fileBuf[0],
			nMaxFile:    uint32(len(fileBuf)),
			// OFN_FILEMUSTEXIST | OFN_NOCHANGEDIR | OFN_EXPLORER
			flags: 0x00001000 | 0x00000008 | 0x00080000,
		}

		ret, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
		if ret != 0 {
			callback(syscall.UTF16ToString(fileBuf))
		}
	}()
}

// openOutputFolder opens the given path in Windows Explorer.
func openOutputFolder(path string) {
	exec.Command("explorer", path).Start() //nolint:errcheck
}

// pickFolder shows the native Windows folder browser dialog.
// callback is called with the selected path; nothing happens if user cancels.
func pickFolder(title string, _ fyne.Window, callback func(string)) {
	go func() {
		// COM must be initialised on the calling thread for SHBrowseForFolder.
		procCoInitializeEx.Call(0, 2) // COINIT_APARTMENTTHREADED
		defer procCoUninitialize.Call()

		titlePtr, _ := syscall.UTF16PtrFromString(title)
		displayBuf := make([]uint16, syscall.MAX_PATH)

		bi := browseInfoW{
			lpszTitle:      titlePtr,
			pszDisplayName: &displayBuf[0],
			// BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE | BIF_EDITBOX
			ulFlags: 0x0001 | 0x0040 | 0x0010,
		}

		pidl, _, _ := procSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
		if pidl == 0 {
			return
		}
		defer procCoTaskMemFree.Call(pidl)

		pathBuf := make([]uint16, syscall.MAX_PATH)
		ret, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
		if ret != 0 {
			callback(syscall.UTF16ToString(pathBuf))
		}
	}()
}
