//go:build !windows

package main

import (
	"context"
	"fmt"
)

func serveGUI(context.Context) error {
	return fmt.Errorf("the native desktop app requires Windows; use host/discover/join here (see --help)")
}
