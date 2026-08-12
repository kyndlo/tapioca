package pythonruntime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Candidate describes one command that may launch a usable Python 3 runtime.
type Candidate struct {
	Name   string
	Prefix []string
}

// Candidates returns platform-appropriate candidates in preference order.
func Candidates(goos string) []Candidate {
	if goos == "windows" {
		return []Candidate{{Name: "py", Prefix: []string{"-3"}}, {Name: "python"}, {Name: "python3"}}
	}
	return []Candidate{{Name: "python3"}, {Name: "python"}}
}

// IsWindowsStoreAlias identifies the placeholder executables installed under
// WindowsApps. Those files may resolve through PATH but cannot run Python.
func IsWindowsStoreAlias(path string, info os.FileInfo) bool {
	clean := strings.ToLower(strings.ReplaceAll(filepath.ToSlash(path), `\`, "/"))
	return strings.Contains(clean, "/microsoft/windowsapps/") ||
		(info != nil && info.Size() == 0)
}

// Find returns a Python 3.10+ interpreter after actually executing each
// candidate. Resolving a name on PATH alone is insufficient on Windows.
func Find(purpose string) (string, []string, error) {
	return findWith(runtime.GOOS, purpose, exec.LookPath, os.Stat, func(path string, args []string) error {
		return exec.Command(path, args...).Run()
	})
}

func findWith(
	goos string,
	purpose string,
	lookPath func(string) (string, error),
	stat func(string) (os.FileInfo, error),
	run func(string, []string) error,
) (string, []string, error) {
	for _, candidate := range Candidates(goos) {
		path, err := lookPath(candidate.Name)
		if err != nil {
			continue
		}
		if goos == "windows" {
			info, statErr := stat(path)
			if statErr != nil || IsWindowsStoreAlias(path, info) {
				continue
			}
		}
		args := append(append([]string{}, candidate.Prefix...),
			"-c", "import sys; raise SystemExit(0 if sys.version_info >= (3, 10) else 1)")
		if err := run(path, args); err == nil {
			return path, append([]string{}, candidate.Prefix...), nil
		}
	}
	if purpose == "" {
		purpose = "this feature"
	}
	return "", nil, fmt.Errorf(
		"no usable Python 3.10+ interpreter was found for %s; install Python from https://python.org and, on Windows, disable Microsoft Store Python aliases if they shadow your installation",
		purpose,
	)
}
