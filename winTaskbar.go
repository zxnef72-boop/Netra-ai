// winTaskbar.go — identitas taskbar untuk jendela app (Edge/Chrome --app=).
// Menulis taskbar.go ke folder stub; stub memanggil taskbarIdentity() setelah membuka browser.
// Efek: jendela memakai AppUserModelID sendiri + perintah buka ulang + ikon dari EXE kita,
// sehingga tombol taskbar tidak bergabung dengan Edge dan pin menyematkan EXE kita.
package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const taskbarHelperSrc = `package main

import (
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const tbAppName = __TB_APPNAME__
const tbAppID = __TB_APPID__

type tbGUID struct {
	D1 uint32
	D2 uint16
	D3 uint16
	D4 [8]byte
}

type tbPropKey struct {
	Fmt tbGUID
	Pid uint32
}

type tbPropVariant struct {
	VT  uint16
	_   [3]uint16
	Ptr uintptr
	_   uintptr
}

var _ = [1]byte{}[unsafe.Sizeof(tbPropVariant{})-24]
var _ = [1]byte{}[unsafe.Sizeof(tbPropKey{})-20]

var (
	tbUser32    = syscall.NewLazyDLL("user32.dll")
	tbShell32   = syscall.NewLazyDLL("shell32.dll")
	tbOle32     = syscall.NewLazyDLL("ole32.dll")
	tbEnumWin   = tbUser32.NewProc("EnumWindows")
	tbGetTitle  = tbUser32.NewProc("GetWindowTextW")
	tbGetClass  = tbUser32.NewProc("GetClassNameW")
	tbIsVisible = tbUser32.NewProc("IsWindowVisible")
	tbPropStore = tbShell32.NewProc("SHGetPropertyStoreForWindow")
	tbCoInit    = tbOle32.NewProc("CoInitializeEx")
	tbCoAlloc   = tbOle32.NewProc("CoTaskMemAlloc")
	tbCoFree    = tbOle32.NewProc("CoTaskMemFree")
)

var tbIIDPropertyStore = tbGUID{0x886D8EEB, 0x8CF2, 0x4446, [8]byte{0x8D, 0x02, 0xCD, 0xBA, 0x1D, 0xBD, 0xCF, 0x99}}
var tbAppModelFmt = tbGUID{0x9F4C2855, 0x9F79, 0x4B39, [8]byte{0xA8, 0xD0, 0xE1, 0xD4, 0x2D, 0xE1, 0xD5, 0xF3}}

var tbWant string
var tbFound uintptr

func tbEnumProc(hwnd uintptr, lparam uintptr) uintptr {
	vis, _, _ := tbIsVisible.Call(hwnd)
	if vis == 0 {
		return 1
	}
	cls := make([]uint16, 64)
	tbGetClass.Call(hwnd, uintptr(unsafe.Pointer(&cls[0])), 64)
	if syscall.UTF16ToString(cls) != "Chrome_WidgetWin_1" {
		return 1
	}
	buf := make([]uint16, 512)
	n, _, _ := tbGetTitle.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), 512)
	if n == 0 {
		return 1
	}
	if strings.HasPrefix(syscall.UTF16ToString(buf[:n]), tbWant) {
		tbFound = hwnd
		return 0
	}
	return 1
}

var tbCallback = syscall.NewCallback(tbEnumProc)

func tbPageTitle(htmlPath string) string {
	b, err := os.ReadFile(htmlPath)
	if err != nil {
		return ""
	}
	s := string(b)
	low := strings.ToLower(s)
	i := strings.Index(low, "<title")
	if i < 0 {
		return ""
	}
	g := strings.Index(low[i:], ">")
	if g < 0 {
		return ""
	}
	st := i + g + 1
	e := strings.Index(low[st:], "</title>")
	if e < 0 {
		return ""
	}
	return strings.TrimSpace(s[st : st+e])
}

func tbSetStr(ps uintptr, pid uint32, val string) bool {
	u, err := syscall.UTF16FromString(val)
	if err != nil {
		return false
	}
	mem, _, _ := tbCoAlloc.Call(uintptr(len(u) * 2))
	if mem == 0 {
		return false
	}
	copy(unsafe.Slice((*uint16)(unsafe.Pointer(mem)), len(u)), u)
	key := tbPropKey{Fmt: tbAppModelFmt, Pid: pid}
	pv := tbPropVariant{VT: 31, Ptr: mem}
	vtbl := *(*[8]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(ps))))
	hr, _, _ := syscall.SyscallN(vtbl[6], ps, uintptr(unsafe.Pointer(&key)), uintptr(unsafe.Pointer(&pv)))
	tbCoFree.Call(mem)
	return hr == 0
}

// taskbarIdentity — tunggu jendela app muncul (cocok dengan <title> HTML), lalu pasang
// AppUserModelID, RelaunchCommand, RelaunchIconResource, RelaunchDisplayNameResource.
func taskbarIdentity(htmlPath string) {
	title := tbPageTitle(htmlPath)
	if title == "" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	tbCoInit.Call(0, 2)
	tbWant = title
	var hwnd uintptr
	for i := 0; i < 75; i++ {
		tbFound = 0
		tbEnumWin.Call(tbCallback, 0)
		if tbFound != 0 {
			hwnd = tbFound
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if hwnd == 0 {
		return
	}
	time.Sleep(300 * time.Millisecond)
	var ps uintptr
	hr, _, _ := tbPropStore.Call(hwnd, uintptr(unsafe.Pointer(&tbIIDPropertyStore)), uintptr(unsafe.Pointer(&ps)))
	if hr != 0 || ps == 0 {
		return
	}
	tbSetStr(ps, 5, tbAppID)
	tbSetStr(ps, 2, "\""+exe+"\"")
	tbSetStr(ps, 3, exe+",0")
	tbSetStr(ps, 4, tbAppName)
	vtbl := *(*[8]uintptr)(unsafe.Pointer(*(*uintptr)(unsafe.Pointer(ps))))
	syscall.SyscallN(vtbl[7], ps)
	syscall.SyscallN(vtbl[2], ps)
}
`

// writeTaskbarHelper — tulis taskbar.go ke folder stub.
func writeTaskbarHelper(dir, appName, safeName string) error {
	src := strings.ReplaceAll(taskbarHelperSrc, "__TB_APPNAME__", strconv.Quote(appName))
	src = strings.ReplaceAll(src, "__TB_APPID__", strconv.Quote("Netra."+safeName))
	return os.WriteFile(filepath.Join(dir, "taskbar.go"), []byte(src), 0644)
}
