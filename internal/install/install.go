package install

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

type Installation struct {
	Data    string
	Engines []string
}

func Detect() []Installation {
	roots := []string{os.Getenv("BAR_DATA_DIR")}
	for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")} {
		if base != "" {
			roots = append(roots, filepath.Join(base, "Beyond-All-Reason", "data"), filepath.Join(base, "Programs", "Beyond-All-Reason", "data"))
		}
	}
	var result []Installation
	seen := map[string]bool{}
	for _, root := range roots {
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		if st, err := os.Stat(root); err == nil && st.IsDir() {
			result = append(result, Installation{root, Engines(root)})
		}
	}
	return result
}
func Engines(data string) []string {
	var result []string
	for _, name := range []string{"spring.exe", "recoil.exe", "spring", "recoil"} {
		matches, _ := filepath.Glob(filepath.Join(data, "engine", "*", name))
		result = append(result, matches...)
	}
	sort.Strings(result)
	return result
}
func Resolve(data, engine string) (string, string, error) {
	if data == "" {
		found := Detect()
		if len(found) != 1 {
			return "", "", fmt.Errorf("found %d data directories; supply --data (see detect)", len(found))
		}
		data = found[0].Data
	}
	data, err := filepath.Abs(data)
	if err != nil {
		return "", "", err
	}
	st, err := os.Stat(data)
	if err != nil {
		return "", "", err
	}
	if !st.IsDir() {
		return "", "", fmt.Errorf("data path must be a directory")
	}
	if engine == "" {
		found := Engines(data)
		if len(found) != 1 {
			return "", "", fmt.Errorf("found %d engines; select --engine explicitly (see detect)", len(found))
		}
		engine = found[0]
	}
	engine, err = filepath.Abs(engine)
	if err != nil {
		return "", "", err
	}
	st, err = os.Stat(engine)
	if err != nil {
		return "", "", err
	}
	if !st.Mode().IsRegular() {
		return "", "", fmt.Errorf("engine must be a regular file")
	}
	return data, engine, nil
}
func Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
