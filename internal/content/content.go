// Package content reads installed archives using the selected engine's unitsync.
// Native DLL calls run in a disposable helper process so faults do not kill the UI.
package content

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Result struct {
	Games        []string `json:"games,omitempty"`
	Maps         []string `json:"maps,omitempty"`
	GameChecksum uint32   `json:"game_checksum,omitempty"`
	MapChecksum  uint32   `json:"map_checksum,omitempty"`
	Bitmap       []byte   `json:"bitmap,omitempty"`
	Error        string   `json:"error,omitempty"`
}

var scanGate = make(chan struct{}, 1)

func Scan(ctx context.Context, data, engine, operation, game, mapName string) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	select {
	case scanGate <- struct{}{}:
		defer func() { <-scanGate }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	out, err := os.CreateTemp("", "bar-lan-content-*.json")
	if err != nil {
		return Result{}, err
	}
	path := out.Name()
	out.Close()
	defer os.Remove(path)
	self, err := os.Executable()
	if err != nil {
		return Result{}, err
	}
	cmd := exec.CommandContext(ctx, self, "content-worker", data, engine, operation, game, mapName, path)
	cmd.Dir = filepath.Dir(engine)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "SPRING_ISOLATED=") && !strings.HasPrefix(e, "SPRING_WRITEDIR=") && !strings.HasPrefix(e, "SPRING_DATADIR=") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "SPRING_ISOLATED="+data, "SPRING_WRITEDIR="+data, "SPRING_DATADIR="+data)
	if err = cmd.Run(); err != nil {
		return Result{}, fmt.Errorf("could not read installed BAR content: %w (check unitsync.dll beside the selected engine)", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	var r Result
	if err = json.Unmarshal(b, &r); err != nil {
		return r, err
	}
	if r.Error != "" {
		return r, fmt.Errorf("%s", r.Error)
	}
	return r, nil
}

// Worker is only called by the disposable helper, before other native work.
func Worker(args []string) error {
	if len(args) != 6 {
		return fmt.Errorf("invalid content helper arguments")
	}
	r, err := readNative(args[0], args[1], args[2], args[3], args[4])
	if err != nil {
		r.Error = err.Error()
	}
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return os.WriteFile(args[5], b, 0600)
}
func SortNames(values []string) {
	sort.Slice(values, func(i, j int) bool { return strings.ToLower(values[i]) < strings.ToLower(values[j]) })
}

// Bitmap565 converts Recoil's top-down RGB565 minimap to a standard 24-bit BMP.
func Bitmap565(pixels []uint16, size int) []byte {
	stride := (size*3 + 3) &^ 3
	b := make([]byte, 54+stride*size)
	copy(b, "BM")
	binary.LittleEndian.PutUint32(b[2:], uint32(len(b)))
	binary.LittleEndian.PutUint32(b[10:], 54)
	binary.LittleEndian.PutUint32(b[14:], 40)
	binary.LittleEndian.PutUint32(b[18:], uint32(size))
	binary.LittleEndian.PutUint32(b[22:], uint32(size))
	binary.LittleEndian.PutUint16(b[26:], 1)
	binary.LittleEndian.PutUint16(b[28:], 24)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			v := pixels[y*size+x]
			o := 54 + (size-1-y)*stride + x*3
			b[o] = byte((v & 31) * 255 / 31)
			b[o+1] = byte(((v >> 5) & 63) * 255 / 63)
			b[o+2] = byte(((v >> 11) & 31) * 255 / 31)
		}
	}
	return b
}
