//go:build !windows

package content

import "fmt"

func readNative(data, engine, operation, game, mapName string) (Result, error) {
	return Result{}, fmt.Errorf("installed-content scanning requires Windows unitsync.dll")
}
