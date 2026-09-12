//go:build windows

package content

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func readNative(data, engine, operation, game, mapName string) (result Result, err error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	path := filepath.Join(filepath.Dir(engine), "unitsync.dll")
	dll, err := syscall.LoadDLL(path)
	if err != nil {
		return result, err
	}
	defer dll.Release()
	names := []string{"Init", "UnInit", "GetMapCount", "GetMapName", "GetPrimaryModCount", "GetPrimaryModInfoCount", "GetInfoKey", "GetInfoValueString", "GetMapChecksumFromName", "GetPrimaryModChecksumFromName", "GetMinimap"}
	api := map[string]*syscall.Proc{}
	for _, n := range names {
		p, e := dll.FindProc(n)
		if e != nil {
			return result, e
		}
		api[n] = p
	}
	call := func(n string, args ...uintptr) uintptr { v, _, _ := api[n].Call(args...); return v }
	kernel := syscall.NewLazyDLL("kernel32.dll")
	process, _, _ := kernel.NewProc("GetCurrentProcess").Call()
	read := kernel.NewProc("ReadProcessMemory")
	copyNative := func(p uintptr, b []byte) bool {
		if p == 0 || len(b) == 0 {
			return false
		}
		v, _, _ := read.Call(process, p, uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)), 0)
		return v != 0
	}
	cstring := func(p uintptr) string {
		if p == 0 {
			return ""
		}
		b := make([]byte, 0, 256)
		one := []byte{0}
		for i := uintptr(0); i < 4096; i++ {
			if !copyNative(p+i, one) {
				return ""
			}
			if one[0] == 0 {
				return string(b)
			}
			b = append(b, one[0])
		}
		return ""
	}
	if call("Init", 0, 0) == 0 {
		return result, fmt.Errorf("unitsync could not initialize this BAR installation")
	}
	defer call("UnInit")
	switch operation {
	case "catalog":
		mods := int32(call("GetPrimaryModCount"))
		maps := int32(call("GetMapCount"))
		if mods < 0 || mods > 100000 || maps < 0 || maps > 100000 {
			return result, fmt.Errorf("unitsync could not list content")
		}
		for i := int32(0); i < mods; i++ {
			count := int32(call("GetPrimaryModInfoCount", uintptr(i)))
			for j := int32(0); j < count; j++ {
				if cstring(call("GetInfoKey", uintptr(j))) == "name" {
					n := cstring(call("GetInfoValueString", uintptr(j)))
					if !strings.Contains(strings.ToLower(n), "chobby") {
						result.Games = append(result.Games, n)
					}
					break
				}
			}
		}
		for i := int32(0); i < maps; i++ {
			result.Maps = append(result.Maps, cstring(call("GetMapName", uintptr(i))))
		}
		SortNames(result.Games)
		SortNames(result.Maps)
	case "checksums", "preview":
		gm, e := syscall.BytePtrFromString(game)
		if e != nil {
			return result, e
		}
		mp, e := syscall.BytePtrFromString(mapName)
		if e != nil {
			return result, e
		}
		if operation == "checksums" {
			result.GameChecksum = uint32(call("GetPrimaryModChecksumFromName", uintptr(unsafe.Pointer(gm))))
			result.MapChecksum = uint32(call("GetMapChecksumFromName", uintptr(unsafe.Pointer(mp))))
			if result.GameChecksum == 0 || result.MapChecksum == 0 {
				return result, fmt.Errorf("the selected game or map is missing or incomplete; download it with BAR first")
			}
		}
		if operation == "preview" {
			p := call("GetMinimap", uintptr(unsafe.Pointer(mp)), 2)
			if p == 0 {
				return result, fmt.Errorf("this map has no readable preview")
			}
			raw := make([]byte, 256*256*2)
			if !copyNative(p, raw) {
				return result, fmt.Errorf("could not read map pixels")
			}
			pixels := make([]uint16, 256*256)
			for i := range pixels {
				pixels[i] = binary.LittleEndian.Uint16(raw[i*2:])
			}
			result.Bitmap = Bitmap565(pixels, 256)
		}
	default:
		return result, fmt.Errorf("unknown content operation")
	}
	return result, nil
}
