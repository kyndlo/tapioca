package pythonruntime

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsCandidatesAvoidStoreAliasFirst(t *testing.T) {
	candidates := Candidates("windows")
	if len(candidates) != 3 || candidates[0].Name != "py" ||
		candidates[1].Name != "python" || candidates[2].Name != "python3" {
		t.Fatalf("Windows candidates = %#v", candidates)
	}
}

func TestWindowsProbeSkipsAliasAndFailedCandidate(t *testing.T) {
	root := t.TempDir()
	alias := filepath.Join(root, "py.exe")
	failed := filepath.Join(root, "python.exe")
	working := filepath.Join(root, "python3.exe")
	if err := os.WriteFile(alias, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{failed, working} {
		if err := os.WriteFile(path, []byte("python"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	paths := map[string]string{"py": alias, "python": failed, "python3": working}
	path, prefix, err := findWith(
		"windows", "tests",
		func(name string) (string, error) { return paths[name], nil },
		os.Stat,
		func(path string, args []string) error {
			if path == failed {
				return errors.New("exit status 9009")
			}
			if path != working || len(args) < 2 {
				return errors.New("unexpected candidate")
			}
			return nil
		},
	)
	if err != nil || path != working || len(prefix) != 0 {
		t.Fatalf("findWith() = %q, %v, %v", path, prefix, err)
	}
}

func TestWindowsStoreAliasDetection(t *testing.T) {
	empty := filepath.Join(t.TempDir(), "python3.exe")
	if err := os.WriteFile(empty, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(empty)
	if err != nil {
		t.Fatal(err)
	}
	if !IsWindowsStoreAlias(empty, info) {
		t.Fatal("zero-byte execution alias was not rejected")
	}
	if !IsWindowsStoreAlias(`C:\Users\person\AppData\Local\Microsoft\WindowsApps\python.exe`, nil) {
		t.Fatal("WindowsApps execution alias was not rejected")
	}
}
