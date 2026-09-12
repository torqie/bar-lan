package install

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
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
	sort.Slice(result, func(i, j int) bool { return Newer(Version(result[i]), Version(result[j])) })
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
		if len(found) == 0 {
			return "", "", fmt.Errorf("no engine installed; open BAR to download it first")
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

func Version(engine string) string { return filepath.Base(filepath.Dir(engine)) }

var versionParts = regexp.MustCompile(`[0-9]+|[^0-9]+`)

// Newer compares numeric build components naturally: build 100 is newer than 99.
func Newer(a, b string) bool {
	aa, bb := versionParts.FindAllString(strings.ToLower(a), -1), versionParts.FindAllString(strings.ToLower(b), -1)
	for i := 0; i < len(aa) && i < len(bb); i++ {
		if aa[i] == bb[i] {
			continue
		}
		an, ae := strconv.ParseUint(aa[i], 10, 64)
		bn, be := strconv.ParseUint(bb[i], 10, 64)
		if ae == nil && be == nil {
			return an > bn
		}
		return aa[i] > bb[i]
	}
	return len(aa) > len(bb)
}
