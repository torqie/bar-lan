package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("BAR_LAN_FAKE_ENGINE"); mode != "" {
		fmt.Println("FAKE_READY")
		if mode == "block" {
			time.Sleep(time.Hour)
			os.Exit(0)
		}
		if len(os.Args) != 5 || os.Args[1] != "--isolation" || os.Args[2] != "--write-dir" {
			os.Exit(2)
		}
		path := os.Args[4]
		b, err := os.ReadFile(path)
		if err != nil {
			os.Exit(3)
		}
		fmt.Printf("SCRIPT_PATH=%s\n%s", path, b)
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func TestEngineLaunchAndTemporaryScriptCleanup(t *testing.T) {
	t.Setenv("BAR_LAN_FAKE_ENGINE", "script")
	engine, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	err = runWithOutput(context.Background(), []string{"join", "--data", t.TempDir(), "--engine", engine, "--host", "127.0.0.1", "--name", "Bob"}, &out)
	if err != nil {
		t.Fatal(err, out.String())
	}
	if !strings.Contains(out.String(), "MyPlayerName=Bob;") {
		t.Fatal(out.String())
	}
	var path string
	for _, line := range strings.Split(out.String(), "\n") {
		if strings.HasPrefix(line, "SCRIPT_PATH=") {
			path = strings.TrimPrefix(line, "SCRIPT_PATH=")
		}
	}
	if path == "" {
		t.Fatal("no script path")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("temporary script remains", err)
	}
}
func TestGUIStopOwnsEngineLifetime(t *testing.T) {
	t.Setenv("BAR_LAN_FAKE_ENGINE", "block")
	engine, _ := os.Executable()
	s := &guiState{}
	r := gameRequest{Mode: "join", Data: t.TempDir(), Engine: engine, Host: "127.0.0.1", Name: "Bob", Port: 8452}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := s.start(ctx, r); err != nil {
		t.Fatal(err)
	}
	if err := s.start(ctx, r); err == nil {
		t.Fatal("second launch accepted")
	}
	waitFor := func(check func() bool) {
		t.Helper()
		until := time.Now().Add(5 * time.Second)
		for time.Now().Before(until) {
			s.mu.Lock()
			ok := check()
			s.mu.Unlock()
			if ok {
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("state timeout")
	}
	waitFor(func() bool { return strings.Contains(s.Log, "FAKE_READY") })
	s.stop()
	waitFor(func() bool { return !s.Running })
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Status != "Stopped" {
		t.Fatal(s.Status, s.Log)
	}
}
