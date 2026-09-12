package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveDoesNotGuessEngine(t *testing.T) {
	d := t.TempDir()
	for _, v := range []string{"v1", "v2"} {
		p := filepath.Join(d, "engine", v)
		if err := os.MkdirAll(p, 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, "spring.exe"), []byte(v), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := Resolve(d, ""); err == nil {
		t.Fatal("ambiguous engines accepted")
	}
	_, e, err := Resolve(d, filepath.Join(d, "engine", "v1", "spring.exe"))
	if err != nil {
		t.Fatal(err)
	}
	a, err := Hash(e)
	if err != nil || len(a) != 64 {
		t.Fatal(a, err)
	}
}
