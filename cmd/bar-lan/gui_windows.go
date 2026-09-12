//go:build windows

package main

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/torqie/bar-lan/internal/install"
	"github.com/torqie/bar-lan/internal/lan"
)

// Standard Win32 controls keep the desktop build self-contained: no browser,
// WebView runtime, C compiler, or third-party GUI framework is required.
var user32 = syscall.NewLazyDLL("user32.dll")
var createWindow = user32.NewProc("CreateWindowExW")
var sendMessage = user32.NewProc("SendMessageW")
var setTextProc = user32.NewProc("SetWindowTextW")
var getTextProc = user32.NewProc("GetWindowTextW")
var postMessage = user32.NewProc("PostMessageW")
var enableWindow = user32.NewProc("EnableWindow")

type windowClass struct {
	Style                              uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
}
type point struct{ X, Y int32 }
type windowMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}
type desktop struct {
	ctx            context.Context
	window, font   uintptr
	controls       map[int]uintptr
	state          guiState
	mu             sync.Mutex
	sessions       []lan.Session
	discoveryError error
	scanning       bool
	lastLog        string
}

const (
	dataID = 101 + iota
	engineID
	detectID
	hostNameID
	guestNameID
	gameID
	mapID
	portID
	hostButtonID
	targetID
	scanID
	sessionsID
	joinButtonID
	ipID
	joinNameID
	manualButtonID
	stopID
	statusID
	logID
)

func wide(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func textOf(h uintptr) string {
	n, _, _ := user32.NewProc("GetWindowTextLengthW").Call(h)
	b := make([]uint16, n+1)
	getTextProc.Call(h, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func setText(h uintptr, s string)      { setTextProc.Call(h, uintptr(unsafe.Pointer(wide(s)))) }
func (d *desktop) value(id int) string { return strings.TrimSpace(textOf(d.controls[id])) }
func (d *desktop) notice(s string) {
	user32.NewProc("MessageBoxW").Call(d.window, uintptr(unsafe.Pointer(wide(s))), uintptr(unsafe.Pointer(wide("BAR LAN"))), 0x40)
}
func (d *desktop) add(class, text string, id, x, y, w, h int, style uintptr) uintptr {
	ex := uintptr(0)
	if class == "EDIT" || class == "LISTBOX" {
		ex = 0x200
	}
	child, _, _ := createWindow.Call(ex, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(text))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), d.window, uintptr(id), 0, 0)
	sendMessage.Call(child, 0x30, d.font, 1)
	if id != 0 {
		d.controls[id] = child
	}
	return child
}
func (d *desktop) label(text string, x, y, w int) { d.add("STATIC", text, 0, x, y, w, 20, 0) }
func (d *desktop) edit(id int, text string, x, y, w int) {
	d.add("EDIT", text, id, x, y, w, 25, 0x10000|0x80)
}
func (d *desktop) button(id int, text string, x, y, w int) {
	d.add("BUTTON", text, id, x, y, w, 30, 0x10000)
}
func (d *desktop) setup() {
	d.label("BAR LAN  /  Local multiplayer", 22, 16, 700)
	d.label("Two PCs. One local match. Both PCs need the same BAR engine, game version and map.", 22, 44, 840)
	d.label("BAR data folder", 22, 80, 700)
	d.add("COMBOBOX", "", dataID, 22, 102, 704, 160, 0x10000|0x200000|2)
	d.button(detectID, "Detect / refresh", 738, 100, 130)
	d.label("Engine executable — select the same build on both PCs", 22, 136, 820)
	d.add("COMBOBOX", "", engineID, 22, 158, 846, 180, 0x10000|0x200000|2)
	d.label("HOST A GAME", 22, 207, 390)
	d.label("Your name", 22, 238, 180)
	d.label("Guest's reserved name", 232, 238, 190)
	d.edit(hostNameID, "Host", 22, 260, 192)
	d.edit(guestNameID, "Guest", 232, 260, 192)
	d.label("Exact installed game name + version", 22, 298, 400)
	d.edit(gameID, "", 22, 320, 402)
	d.label("Exact installed map name (.smf)", 22, 358, 400)
	d.edit(mapID, "", 22, 380, 402)
	d.label("Game UDP port (also used for manual joining)", 22, 418, 405)
	d.edit(portID, "8452", 22, 440, 100)
	d.button(hostButtonID, "Host game", 22, 482, 402)
	d.label("JOIN A GAME", 464, 207, 400)
	d.label("Discovery target (broadcast or host IPv4)", 464, 238, 404)
	d.edit(targetID, "255.255.255.255", 464, 260, 265)
	d.button(scanID, "Find games", 741, 258, 127)
	d.add("LISTBOX", "", sessionsID, 464, 300, 404, 110, 0x10000|0x200000|1)
	d.button(joinButtonID, "Join selected game", 464, 422, 404)
	d.label("Manual host IP", 464, 468, 200)
	d.label("Reserved guest name", 674, 468, 194)
	d.edit(ipID, "", 464, 490, 192)
	d.edit(joinNameID, "Guest", 674, 490, 194)
	d.button(manualButtonID, "Join by IP", 464, 529, 404)
	d.controls[statusID] = d.add("STATIC", "Ready", statusID, 22, 575, 680, 22, 0)
	d.button(stopID, "Stop game", 738, 570, 130)
	d.add("EDIT", "Launch details will appear here.", logID, 22, 610, 846, 112, 0x10000|0x200000|0x4|0x40|0x800)
	d.detect(true)
	user32.NewProc("SetTimer").Call(d.window, 1, 500, 0)
}
func (d *desktop) detect(initial bool) {
	if initial || d.value(dataID) == "" {
		found := install.Detect()
		sendMessage.Call(d.controls[dataID], 0x14b, 0, 0)
		for _, f := range found {
			sendMessage.Call(d.controls[dataID], 0x143, 0, uintptr(unsafe.Pointer(wide(f.Data))))
		}
		if len(found) > 0 {
			sendMessage.Call(d.controls[dataID], 0x14e, 0, 0)
		}
	}
	engines := install.Engines(d.value(dataID))
	sendMessage.Call(d.controls[engineID], 0x14b, 0, 0)
	for _, e := range engines {
		sendMessage.Call(d.controls[engineID], 0x143, 0, uintptr(unsafe.Pointer(wide(e))))
	}
	if len(engines) == 1 {
		sendMessage.Call(d.controls[engineID], 0x14e, 0, 0)
	} else {
		setText(d.controls[engineID], "")
	}
	if !initial && len(engines) == 0 {
		d.notice("No engine found. Enter your BAR data folder, then click Detect / refresh. You can also paste the full engine executable path.")
	}
}
func (d *desktop) request() (gameRequest, error) {
	port, err := strconv.Atoi(d.value(portID))
	if err != nil {
		return gameRequest{}, fmt.Errorf("enter a numeric game port")
	}
	return gameRequest{Data: d.value(dataID), Engine: d.value(engineID), Port: port}, nil
}
func (d *desktop) launch(r gameRequest) {
	if err := d.state.start(d.ctx, r); err != nil {
		d.notice(err.Error())
	}
	d.tick()
}
func (d *desktop) command(id int) {
	switch id {
	case detectID:
		d.detect(false)
	case scanID:
		d.mu.Lock()
		if d.scanning {
			d.mu.Unlock()
			return
		}
		d.scanning = true
		d.mu.Unlock()
		target := d.value(targetID)
		setText(d.controls[scanID], "Searching...")
		enableWindow.Call(d.controls[scanID], 0)
		go func() {
			sessions, err := lan.Discover(d.ctx, target, 3e9)
			d.mu.Lock()
			d.sessions = sessions
			d.discoveryError = err
			d.scanning = false
			d.mu.Unlock()
			postMessage.Call(d.window, 0x8001, 0, 0)
		}()
	case hostButtonID:
		r, err := d.request()
		if err != nil {
			d.notice(err.Error())
			return
		}
		r.Mode = "host"
		r.Name = d.value(hostNameID)
		r.Guest = d.value(guestNameID)
		r.Game = d.value(gameID)
		r.Map = d.value(mapID)
		d.launch(r)
	case joinButtonID:
		selected, _, _ := sendMessage.Call(d.controls[sessionsID], 0x188, 0, 0)
		d.mu.Lock()
		if selected >= uintptr(len(d.sessions)) {
			d.mu.Unlock()
			d.notice("Find games and select a host first.")
			return
		}
		s := d.sessions[selected]
		d.mu.Unlock()
		r, err := d.request()
		if err != nil {
			d.notice(err.Error())
			return
		}
		r.Mode = "join"
		r.Target = s.IP
		r.Name = s.Guest
		d.launch(r)
	case manualButtonID:
		r, err := d.request()
		if err != nil {
			d.notice(err.Error())
			return
		}
		r.Mode = "join"
		r.Host = d.value(ipID)
		r.Name = d.value(joinNameID)
		d.launch(r)
	case stopID:
		d.state.stop()
		d.tick()
	}
}
func (d *desktop) discovered() {
	setText(d.controls[scanID], "Find games")
	enableWindow.Call(d.controls[scanID], 1)
	sendMessage.Call(d.controls[sessionsID], 0x184, 0, 0)
	d.mu.Lock()
	sessions := append([]lan.Session(nil), d.sessions...)
	err := d.discoveryError
	d.mu.Unlock()
	if err != nil {
		d.notice(err.Error())
		return
	}
	for _, s := range sessions {
		label := fmt.Sprintf("%s  |  %s  |  %s", s.IP, s.Host, s.Map)
		sendMessage.Call(d.controls[sessionsID], 0x180, 0, uintptr(unsafe.Pointer(wide(label))))
	}
	if len(sessions) > 0 {
		sendMessage.Call(d.controls[sessionsID], 0x186, 0, 0)
	} else {
		d.notice("No games found. Try the host's IPv4 address as the discovery target, or Join by IP. Check Private-network firewall rules on the host.")
	}
}
func (d *desktop) tick() {
	d.state.mu.Lock()
	running, status, log := d.state.Running, d.state.Status, d.state.Log
	d.state.mu.Unlock()
	if running && strings.Contains(log, "Recoil started.") {
		status = "Recoil is running — follow the in-game ready / start controls"
	}
	setText(d.controls[statusID], status)
	if log != d.lastLog {
		d.lastLog = log
		setText(d.controls[logID], strings.ReplaceAll(log, "\n", "\r\n"))
		sendMessage.Call(d.controls[logID], 0xb1, ^uintptr(0), ^uintptr(0))
		sendMessage.Call(d.controls[logID], 0xb7, 0, 0)
	}
	idle := uintptr(1)
	active := uintptr(0)
	if running {
		idle = 0
		active = 1
	}
	for _, id := range []int{hostButtonID, joinButtonID, manualButtonID} {
		enableWindow.Call(d.controls[id], idle)
	}
	enableWindow.Call(d.controls[stopID], active)
}
func serveGUI(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	d := &desktop{ctx: ctx, controls: map[int]uintptr{}, state: guiState{Status: "Ready"}}
	instance, _, _ := syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := user32.NewProc("LoadCursorW").Call(0, 32512)
	proc := syscall.NewCallback(func(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
		switch msg {
		case 0x111:
			id := int(w & 0xffff)
			notification := w >> 16
			if id == dataID && notification == 1 { // CBN_SELCHANGE precedes edit text update.
				postMessage.Call(hwnd, 0x8002, 0, 0)
				return 0
			}
			if notification == 0 {
				d.command(id)
			}
			return 0
		case 0x113:
			d.tick()
			return 0
		case 0x8001:
			d.discovered()
			return 0
		case 0x8002:
			d.detect(false)
			return 0
		case 0x10:
			d.state.mu.Lock()
			running := d.state.Running
			d.state.mu.Unlock()
			if running {
				d.notice("Exit Recoil or use Stop game before closing BAR LAN.")
				return 0
			}
			user32.NewProc("DestroyWindow").Call(hwnd)
			return 0
		case 2:
			user32.NewProc("PostQuitMessage").Call(0)
			return 0
		}
		r, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), w, l)
		return r
	})
	class := windowClass{Proc: proc, Instance: instance, Cursor: cursor, Background: 16, Name: wide("BarLanDesktop")}
	atom, _, err := user32.NewProc("RegisterClassW").Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return fmt.Errorf("register desktop: %w", err)
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(class.Name)), instance)
	d.font, _, _ = syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject").Call(17)
	// Fixed client layout, system DPI virtualization, standard keyboard navigation.
	d.window, _, err = createWindow.Call(0x10000, uintptr(unsafe.Pointer(class.Name)), uintptr(unsafe.Pointer(wide("BAR LAN — Local multiplayer"))), 0x00c80000, 0x80000000, 0x80000000, 910, 775, 0, 0, instance, 0)
	if d.window == 0 {
		return fmt.Errorf("create desktop: %w", err)
	}
	d.setup()
	user32.NewProc("ShowWindow").Call(d.window, 1)
	user32.NewProc("UpdateWindow").Call(d.window)
	var message windowMessage
	for {
		ret, _, err := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		if int32(ret) == -1 {
			return err
		}
		if ret == 0 {
			break
		}
		handled, _, _ := user32.NewProc("IsDialogMessageW").Call(d.window, uintptr(unsafe.Pointer(&message)))
		if handled == 0 {
			user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&message)))
			user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&message)))
		}
	}
	return nil
}
