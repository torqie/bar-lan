package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSelectsNewestEngine(t *testing.T) {
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
	if _, e, err := Resolve(d, ""); err != nil || Version(e) != "v2" {
		t.Fatal("newest engine not selected", e, err)
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

func TestEngineBuildOrdering(t *testing.T) {
	for _, pair := range [][2]string{{"105.1.1-100-gabc BAR", "105.1.1-99-gdef BAR"}, {"2026.10.1", "2026.9.30"}, {"v10", "v9"}} {
		if !Newer(pair[0], pair[1]) || Newer(pair[1], pair[0]) {
			t.Fatal(pair)
		}
	}
}
