//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/torqie/bar-lan/internal/content"
	"github.com/torqie/bar-lan/internal/install"
	"github.com/torqie/bar-lan/internal/lan"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var createWindow = user32.NewProc("CreateWindowExW")
var sendMessage = user32.NewProc("SendMessageW")
var setTextProc = user32.NewProc("SetWindowTextW")
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
type preferences struct{ Name, Data string }
type desktop struct {
	ctx                                  context.Context
	window, home, font, instance, bitmap uintptr
	class                                *uint16
	controls, homeControls               map[int]uintptr
	mode, data, engine, name             string
	catalog                              content.Result
	engines                              []string
	lobby                                lobby
	mu                                   sync.Mutex
	generation                           uint64
	loaded                               bool
	loadResult                           content.Result
	loadErr                              error
	sessions                             []lan.Session
	scanErr                              error
	scanDone                             bool
	preview                              []byte
	previewErr                           error
	previewDone                          bool
	previewGeneration                    uint64
	previewCancel                        context.CancelFunc
	lastLog                              string
}

const (
	nameID = 101 + iota
	hostID
	joinID
	settingsID
	dataID
	engineID
	applyID
	backID
	versionID
	gameID
	mapID
	previewID
	previewTextID
	openID
	startID
	targetID
	scanID
	listID
	connectID
	roomID
	leaveID
	logID
	detailsID
)

func wide(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }
func textOf(h uintptr) string {
	if h == 0 {
		return ""
	}
	n, _, _ := user32.NewProc("GetWindowTextLengthW").Call(h)
	b := make([]uint16, n+1)
	user32.NewProc("GetWindowTextW").Call(h, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
	return syscall.UTF16ToString(b)
}
func setText(h uintptr, s string) {
	if h != 0 {
		setTextProc.Call(h, uintptr(unsafe.Pointer(wide(s))))
	}
}
func (d *desktop) value(id int) string { return strings.TrimSpace(textOf(d.controls[id])) }
func (d *desktop) notice(s string) {
	user32.NewProc("MessageBoxW").Call(d.window, uintptr(unsafe.Pointer(wide(s))), uintptr(unsafe.Pointer(wide("BAR LAN"))), 0x40)
}
func (d *desktop) add(class, text string, id, x, y, w, h int, style uintptr) uintptr {
	ex := uintptr(0)
	if class == "EDIT" || class == "LISTBOX" {
		ex = 0x200
	}
	v, _, _ := createWindow.Call(ex, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(text))), 0x50000000|style, uintptr(x), uintptr(y), uintptr(w), uintptr(h), d.window, uintptr(id), d.instance, 0)
	sendMessage.Call(v, 0x30, d.font, 1)
	if id != 0 {
		d.controls[id] = v
	}
	return v
}
func (d *desktop) label(s string, x, y, w int)          { d.add("STATIC", s, 0, x, y, w, 22, 0) }
func (d *desktop) button(id int, s string, x, y, w int) { d.add("BUTTON", s, id, x, y, w, 36, 0x10000) }
func (d *desktop) edit(id int, s string, x, y, w int) {
	d.add("EDIT", s, id, x, y, w, 28, 0x10000|0x80)
}
func (d *desktop) combo(id, x, y, w int) { d.add("COMBOBOX", "", id, x, y, w, 260, 0x10000|0x200000|3) }
func (d *desktop) fill(id int, values []string) {
	h := d.controls[id]
	if h == 0 {
		return
	}
	sendMessage.Call(h, 0x14b, 0, 0)
	for _, s := range values {
		sendMessage.Call(h, 0x143, 0, uintptr(unsafe.Pointer(wide(s))))
	}
	if len(values) > 0 {
		sendMessage.Call(h, 0x14e, 0, 0)
	}
}
func enabled(h uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	enableWindow.Call(h, v)
}
func (d *desktop) engineLabel() string {
	if d.engine == "" {
		return "BAR engine not found — open Installation settings"
	}
	return "Engine: " + install.Version(d.engine) + "  (newest installed selected automatically)"
}
func settingsPath() string {
	base, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "bar-lan", "settings.json")
}
func (d *desktop) save() {
	p := settingsPath()
	if p == "" {
		return
	}
	b, _ := json.Marshal(preferences{d.name, d.data})
	if os.MkdirAll(filepath.Dir(p), 0700) == nil {
		_ = os.WriteFile(p, b, 0600)
	}
}
func (d *desktop) homeSetup() {
	d.label("BAR LAN", 28, 24, 700)
	d.label("Play Beyond All Reason together on your local network.", 28, 56, 720)
	d.label("Your name", 28, 106, 700)
	d.edit(nameID, d.name, 28, 132, 720)
	d.button(hostID, "Host Game", 28, 194, 348)
	d.button(joinID, "Join Game", 400, 194, 348)
	d.add("STATIC", d.engineLabel(), versionID, 28, 260, 720, 48, 0)
	d.label("The host chooses a map. Your friend joins with their own name.", 28, 320, 730)
	d.label("BAR LAN checks both PCs before the host starts the match.", 28, 348, 730)
	d.button(settingsID, "Installation settings", 28, 405, 260)
	d.homeControls = d.controls
}
func (d *desktop) workspace(mode string) {
	if d.mode == "" {
		d.name = d.value(nameID)
		if mode != "settings" {
			if err := validatePlayer(d.name); err != nil {
				d.notice(err.Error())
				return
			}
		}
		d.save()
	}
	d.lobby.mu.Lock()
	d.lobby.view.Status = ""
	d.lobby.mu.Unlock()
	d.mode = mode
	d.controls = map[int]uintptr{}
	title := "BAR LAN — " + map[string]string{"host": "Host Game", "join": "Join Game", "settings": "Installation settings"}[mode]
	window, _, err := createWindow.Call(0x10000, uintptr(unsafe.Pointer(d.class)), uintptr(unsafe.Pointer(wide(title))), 0x00c80000, 0x80000000, 0x80000000, 800, 800, d.home, 0, d.instance, 0)
	if window == 0 {
		d.mode = ""
		d.controls = d.homeControls
		d.notice(err.Error())
		return
	}
	d.window = window
	user32.NewProc("ShowWindow").Call(d.home, 0)
	d.button(backID, "Back", 650, 18, 110)
	if mode == "settings" {
		d.label("Installation settings", 24, 26, 600)
		d.label("BAR data folder", 24, 92, 700)
		d.edit(dataID, d.data, 24, 118, 736)
		d.label("Engine build — newest installed is selected by default", 24, 174, 730)
		d.combo(engineID, 24, 202, 736)
		d.engines = install.Engines(d.data)
		versions := []string{}
		for _, e := range d.engines {
			versions = append(versions, install.Version(e))
		}
		d.fill(engineID, versions)
		d.button(applyID, "Apply / scan folder", 24, 256, 300)
		d.label("After changing the folder, click Apply / scan folder to select its latest engine.", 24, 316, 736)
	} else {
		d.label(map[string]string{"host": "Host Game", "join": "Join Game"}[mode]+"  ·  "+d.name, 24, 26, 600)
		d.add("STATIC", d.engineLabel(), versionID, 24, 76, 736, 42, 0)
		if mode == "host" {
			d.label("Game", 24, 135, 430)
			d.combo(gameID, 24, 160, 430)
			d.label("Map", 24, 211, 430)
			d.combo(mapID, 24, 237, 430)
			d.add("STATIC", "", previewID, 490, 135, 256, 256, 0xe)
			d.add("STATIC", "Select a map to see its preview", previewTextID, 490, 400, 256, 46, 0)
			d.button(openID, "Open room", 24, 306, 430)
			d.button(startID, "Start game together", 24, 362, 430)
		} else {
			d.label("Find a friend's game on your network", 24, 135, 730)
			d.edit(targetID, "255.255.255.255", 24, 164, 470)
			d.button(scanID, "Find games", 516, 160, 244)
			d.label("If needed, replace the address above with your friend's local IP.", 24, 209, 730)
			d.add("LISTBOX", "", listID, 24, 242, 736, 112, 0x10000|0x200000|1)
			d.add("STATIC", "Select a room to compare versions and see its map.", detailsID, 24, 370, 440, 86, 0)
			d.button(connectID, "Join selected room", 24, 458, 430)
			d.add("STATIC", "", previewID, 504, 370, 256, 256, 0xe)
		}
		d.add("STATIC", "Reading your installed games and maps...", roomID, 24, modeY(mode, 530, 508), modeY(mode, 736, 450), 64, 0)
		d.button(leaveID, "Leave room / stop game", 24, modeY(mode, 602, 584), 430)
		d.add("EDIT", "", logID, 24, 660, 736, 80, 0x10000|0x200000|4|0x40|0x800)
		d.loadCatalog()
	}
	user32.NewProc("ShowWindow").Call(d.window, 1)
	user32.NewProc("SetTimer").Call(d.window, 1, 300, 0)
}
func modeY(mode string, host, join int) int {
	if mode == "host" {
		return host
	}
	return join
}
func (d *desktop) back() {
	d.lobby.game.mu.Lock()
	running := d.lobby.game.Running
	d.lobby.game.mu.Unlock()
	if running {
		d.notice("Exit Recoil or use Leave room / stop game first.")
		return
	}
	d.lobby.close()
	if d.previewCancel != nil {
		d.previewCancel()
	}
	d.mu.Lock()
	d.generation++
	d.previewGeneration++
	d.mu.Unlock()
	old := d.window
	d.window = d.home
	d.mode = ""
	d.controls = d.homeControls
	user32.NewProc("DestroyWindow").Call(old)
	if d.bitmap != 0 {
		syscall.NewLazyDLL("gdi32.dll").NewProc("DeleteObject").Call(d.bitmap)
		d.bitmap = 0
	}
	setText(d.controls[versionID], d.engineLabel())
	user32.NewProc("ShowWindow").Call(d.home, 1)
}
func (d *desktop) loadCatalog() {
	d.catalog = content.Result{}
	d.mu.Lock()
	d.generation++
	generation := d.generation
	d.loaded = false
	d.loadErr = nil
	d.mu.Unlock()
	data, engine := d.data, d.engine
	enabled(d.controls[openID], false)
	enabled(d.controls[connectID], false)
	go func() {
		result, err := content.Scan(d.ctx, data, engine, "catalog", "", "")
		d.mu.Lock()
		defer d.mu.Unlock()
		if generation == d.generation {
			d.loadResult = result
			d.loadErr = err
			d.loaded = true
		}
	}()
}
func (d *desktop) mapPreview(name string) {
	if d.previewCancel != nil {
		d.previewCancel()
	}
	ctx, cancel := context.WithCancel(d.ctx)
	d.previewCancel = cancel
	sendMessage.Call(d.controls[previewID], 0x172, 0, 0)
	if d.bitmap != 0 {
		syscall.NewLazyDLL("gdi32.dll").NewProc("DeleteObject").Call(d.bitmap)
		d.bitmap = 0
	}
	setText(d.controls[previewTextID], "Loading map preview...")
	d.mu.Lock()
	d.previewGeneration++
	d.previewDone = false
	g := d.previewGeneration
	d.mu.Unlock()
	data, engine := d.data, d.engine
	go func() {
		r, err := content.Scan(ctx, data, engine, "preview", "", name)
		d.mu.Lock()
		defer d.mu.Unlock()
		if g == d.previewGeneration {
			d.preview = r.Bitmap
			d.previewErr = err
			d.previewDone = true
		}
	}()
}
func (d *desktop) showPreview(b []byte) {
	f, err := os.CreateTemp("", "bar-lan-map-*.bmp")
	if err != nil {
		return
	}
	p := f.Name()
	defer os.Remove(p)
	_, err = f.Write(b)
	f.Close()
	if err != nil {
		return
	}
	h, _, _ := user32.NewProc("LoadImageW").Call(0, uintptr(unsafe.Pointer(wide(p))), 0, 256, 256, 0x10)
	if h == 0 {
		return
	}
	old, _, _ := sendMessage.Call(d.controls[previewID], 0x172, 0, h)
	if old != 0 {
		syscall.NewLazyDLL("gdi32.dll").NewProc("DeleteObject").Call(old)
	}
	d.bitmap = h
	setText(d.controls[previewTextID], "Map preview")
}
func (d *desktop) selected() (lan.Session, bool) {
	i, _, _ := sendMessage.Call(d.controls[listID], 0x188, 0, 0)
	d.mu.Lock()
	defer d.mu.Unlock()
	if i >= uintptr(len(d.sessions)) {
		return lan.Session{}, false
	}
	return d.sessions[i], true
}
func (d *desktop) selection() {
	s, ok := d.selected()
	if !ok {
		return
	}
	match := "Engine build differs"
	if s.EngineVersion == install.Version(d.engine) {
		match = "Same engine build (files checked when joining)"
	}
	setText(d.controls[detailsID], fmt.Sprintf("Host: %s\r\n%s\r\nGame: %s\r\nMap: %s", s.Host, match, s.Game, s.Map))
	d.mapPreview(s.Map)
}
func (d *desktop) command(id int) {
	switch id {
	case hostID:
		d.workspace("host")
	case joinID:
		d.workspace("join")
	case settingsID:
		d.workspace("settings")
	case backID:
		d.back()
	case applyID:
		data := d.value(dataID)
		engine := ""
		if data == d.data {
			i, _, _ := sendMessage.Call(d.controls[engineID], 0x147, 0, 0)
			if i < uintptr(len(d.engines)) {
				engine = d.engines[i]
			}
		}
		data, engine, err := install.Resolve(data, engine)
		if err != nil {
			d.notice(err.Error())
			return
		}
		d.data = data
		d.engine = engine
		d.save()
		d.back()
	case openID:
		if d.previewCancel != nil {
			d.previewCancel()
		}
		r := gameRequest{Mode: "host", Data: d.data, Engine: d.engine, Name: d.name, Game: d.value(gameID), Map: d.value(mapID), Port: 8452}
		if r.Game == "" || r.Map == "" {
			d.notice("Choose an installed game and map first.")
			return
		}
		if err := d.lobby.hostRoom(d.ctx, r); err != nil {
			d.notice(err.Error())
		}
	case startID:
		if err := d.lobby.startHost(); err != nil {
			d.notice(err.Error())
		}
	case leaveID:
		d.lobby.close()
	case scanID:
		enabled(d.controls[scanID], false)
		setText(d.controls[scanID], "Searching...")
		target := d.value(targetID)
		d.mu.Lock()
		g := d.generation
		d.mu.Unlock()
		go func() {
			s, err := lan.Discover(d.ctx, target, 3*time.Second)
			filtered := []lan.Session{}
			for _, v := range s {
				if v.Version == 2 && v.Phase == "waiting" {
					filtered = append(filtered, v)
				}
			}
			d.mu.Lock()
			defer d.mu.Unlock()
			if g == d.generation {
				d.sessions = filtered
				d.scanErr = err
				d.scanDone = true
			}
		}()
	case connectID:
		if d.previewCancel != nil {
			d.previewCancel()
		}
		s, ok := d.selected()
		if !ok {
			d.notice("Find games and select your friend's room first.")
			return
		}
		if err := d.lobby.joinRoom(d.ctx, s, gameRequest{Data: d.data, Engine: d.engine, Name: d.name}); err != nil {
			d.notice(err.Error())
		}
	}
}
func (d *desktop) tick() {
	if d.mode == "" || d.mode == "settings" {
		return
	}
	d.mu.Lock()
	loaded, r, loadErr := d.loaded, d.loadResult, d.loadErr
	d.loaded = false
	scanDone, sessions, scanErr := d.scanDone, append([]lan.Session(nil), d.sessions...), d.scanErr
	d.scanDone = false
	previewDone, preview, previewErr := d.previewDone, d.preview, d.previewErr
	d.previewDone = false
	d.mu.Unlock()
	if loaded {
		d.catalog = r
		if loadErr != nil {
			setText(d.controls[roomID], "Could not read installed content. "+loadErr.Error())
		} else {
			// Prefer the latest BAR game; maps are alphabetic. Users never type archive names.
			games := append([]string(nil), r.Games...)
			sortGames(games)
			d.fill(gameID, games)
			d.fill(mapID, r.Maps)
			if len(games) == 0 || len(r.Maps) == 0 {
				setText(d.controls[roomID], "No games or maps found. Open BAR, download content, then return here.")
			} else {
				setText(d.controls[roomID], "Choose a map and open your room.")
				if d.mode == "host" {
					d.mapPreview(d.value(mapID))
				} else {
					setText(d.controls[roomID], "Ready — find your friend's room.")
					d.command(scanID)
				}
			}
		}
	}
	if scanDone {
		enabled(d.controls[scanID], true)
		setText(d.controls[scanID], "Find games")
		sendMessage.Call(d.controls[listID], 0x184, 0, 0)
		for _, s := range sessions {
			label := s.Host + "  |  " + s.Map + "  |  " + s.EngineVersion
			sendMessage.Call(d.controls[listID], 0x180, 0, uintptr(unsafe.Pointer(wide(label))))
		}
		if len(sessions) > 0 {
			sendMessage.Call(d.controls[listID], 0x186, 0, 0)
			d.selection()
		} else {
			setText(d.controls[roomID], "No open rooms found. Ask your friend to open a room, or enter their local IP above.")
		}
		if scanErr != nil {
			setText(d.controls[roomID], scanErr.Error())
		}
	}
	if previewDone {
		if previewErr == nil {
			d.showPreview(preview)
		} else {
			setText(d.controls[previewTextID], "Preview unavailable for this map")
		}
	}
	v := d.lobby.snapshot()
	if v.Status != "" {
		text := v.Status
		if v.Peer != "" {
			text += "\r\nPlayer: " + v.Peer + "  |  Engine: " + v.Engine
		}
		setText(d.controls[roomID], text)
	}
	d.lobby.game.mu.Lock()
	running, log := d.lobby.game.Running, d.lobby.game.Log
	d.lobby.game.mu.Unlock()
	if log != d.lastLog {
		d.lastLog = log
		setText(d.controls[logID], strings.ReplaceAll(log, "\n", "\r\n"))
	}
	ready := len(d.catalog.Games) > 0 && len(d.catalog.Maps) > 0
	enabled(d.controls[openID], ready && !v.Active && !running)
	enabled(d.controls[connectID], ready && !v.Active && !running)
	enabled(d.controls[startID], v.CanStart && !running)
	enabled(d.controls[listID], !v.Active)
	enabled(d.controls[targetID], !v.Active)
	enabled(d.controls[gameID], !v.Active)
	enabled(d.controls[mapID], !v.Active)
	enabled(d.controls[leaveID], v.Active || running)
}
func serveGUI(ctx context.Context) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	d := &desktop{ctx: ctx, controls: map[int]uintptr{}}
	var prefs preferences
	if b, err := os.ReadFile(settingsPath()); err == nil {
		_ = json.Unmarshal(b, &prefs)
	}
	d.name = prefs.Name
	d.data = prefs.Data
	if d.data == "" {
		found := install.Detect()
		if len(found) > 0 {
			d.data = found[0].Data
		}
	}
	if data, engine, err := install.Resolve(d.data, ""); err == nil {
		d.data = data
		d.engine = engine
	}
	d.instance, _, _ = syscall.NewLazyDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := user32.NewProc("LoadCursorW").Call(0, 32512)
	callback := syscall.NewCallback(func(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
		switch msg {
		case 0x111:
			id, n := int(w&0xffff), w>>16
			if (id == mapID || id == listID) && n == 1 {
				postMessage.Call(hwnd, 0x8002, uintptr(id), 0)
				return 0
			}
			if n == 0 {
				d.command(id)
			}
			return 0
		case 0x8002:
			if int(w) == mapID {
				d.mapPreview(d.value(mapID))
			} else {
				d.selection()
			}
			return 0
		case 0x113:
			d.tick()
			return 0
		case 0x10:
			if hwnd != d.home {
				d.back()
				return 0
			}
			d.name = d.value(nameID)
			d.save()
			user32.NewProc("DestroyWindow").Call(hwnd)
			return 0
		case 2:
			if hwnd == d.home {
				user32.NewProc("PostQuitMessage").Call(0)
			}
			return 0
		}
		v, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), w, l)
		return v
	})
	d.class = wide("BarLanDesktopV2")
	wc := windowClass{Proc: callback, Instance: d.instance, Cursor: cursor, Background: 16, Name: d.class}
	if v, _, err := user32.NewProc("RegisterClassW").Call(uintptr(unsafe.Pointer(&wc))); v == 0 {
		return err
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(d.class)), d.instance)
	d.font, _, _ = syscall.NewLazyDLL("gdi32.dll").NewProc("GetStockObject").Call(17)
	var err error
	d.home, _, err = createWindow.Call(0x10000, uintptr(unsafe.Pointer(d.class)), uintptr(unsafe.Pointer(wide("BAR LAN"))), 0x00c80000, 0x80000000, 0x80000000, 800, 510, 0, 0, d.instance, 0)
	if d.home == 0 {
		return err
	}
	d.window = d.home
	d.homeSetup()
	user32.NewProc("ShowWindow").Call(d.home, 1)
	var m windowMessage
	for {
		v, _, err := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(v) == -1 {
			return err
		}
		if v == 0 {
			break
		}
		handled, _, _ := user32.NewProc("IsDialogMessageW").Call(d.window, uintptr(unsafe.Pointer(&m)))
		if handled == 0 {
			user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&m)))
			user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&m)))
		}
	}
	d.lobby.close()
	return nil
}
