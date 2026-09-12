package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDryRunAndValidation(t *testing.T) {
	d := t.TempDir()
	e := filepath.Join(d, "engine with spaces.exe")
	if err := os.WriteFile(e, []byte("not a real executable"), 0600); err != nil {
		t.Fatal(err)
	}
	common := []string{"--data", d, "--engine", e, "--dry-run"}
	cases := []struct {
		args []string
		bad  bool
	}{
		{[]string{"host", "--game", "BAR version", "--map", "Map.smf"}, false},
		{[]string{"host", "--game", "BAR", "--map", "Map.smf", "--port", "8453"}, true},
		{[]string{"host", "--game", "BAR;bad", "--map", "Map.smf"}, true},
		{[]string{"join", "--host", "192.168.1.20", "--name", "Bob"}, false},
		{[]string{"join", "--host", "192.168.1.20"}, true},
	}
	for _, c := range cases {
		err := run(context.Background(), append(c.args, common...))
		if (err != nil) != c.bad {
			t.Errorf("%v: %v", c.args, err)
		}
	}
}
