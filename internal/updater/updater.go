package updater

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const defaultReleaseAPI = "https://api.github.com/repos/kyndlo/tapioca/releases/latest"

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type releaseResponse struct {
	TagName string  `json:"tag_name"`
	URL     string  `json:"html_url"`
	Draft   bool    `json:"draft"`
	Assets  []Asset `json:"assets"`
}

type Info struct {
	Current       string
	Latest        string
	Available     bool
	ReleaseURL    string
	Asset         Asset
	ChecksumAsset Asset
}

type Result struct {
	Info
	Executable string
}

func Check(ctx context.Context, current string) (Info, error) {
	api := os.Getenv("TAPIOCA_RELEASE_API")
	if api == "" {
		api = defaultReleaseAPI
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	if err != nil {
		return Info{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "tapioca/"+current)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Info{}, fmt.Errorf("GitHub release check failed: %s", resp.Status)
	}
	var release releaseResponse
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 2<<20))
	if err := decoder.Decode(&release); err != nil {
		return Info{}, err
	}
	latest := strings.TrimPrefix(strings.TrimSpace(release.TagName), "v")
	if release.Draft || !validVersion(latest) {
		return Info{}, errors.New("latest GitHub release has an invalid version")
	}
	current = strings.TrimPrefix(strings.TrimSpace(current), "v")
	if !validVersion(current) {
		return Info{}, fmt.Errorf("current Tapioca version %q is invalid", current)
	}
	available := compareVersions(latest, current) > 0
	info := Info{Current: current, Latest: latest, Available: available, ReleaseURL: release.URL}
	if !available {
		return info, nil
	}
	assetName, err := cliAssetName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return Info{}, err
	}
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			info.Asset = asset
		case assetName + ".sha256":
			info.ChecksumAsset = asset
		}
	}
	if info.Asset.URL == "" || info.ChecksumAsset.URL == "" {
		return Info{}, fmt.Errorf("release v%s does not contain %s and its checksum", latest, assetName)
	}
	return info, nil
}

func Apply(ctx context.Context, current string) (Result, error) {
	info, err := Check(ctx, current)
	if err != nil || !info.Available {
		return Result{Info: info}, err
	}
	temporary, err := os.MkdirTemp("", "tapioca-update-")
	if err != nil {
		return Result{}, err
	}
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.RemoveAll(temporary)
		}
	}()
	archive := filepath.Join(temporary, info.Asset.Name)
	if err := download(ctx, info.Asset.URL, archive, 2<<30); err != nil {
		return Result{}, fmt.Errorf("download update: %w", err)
	}
	checksumData, err := downloadBytes(ctx, info.ChecksumAsset.URL, 4096)
	if err != nil {
		return Result{}, fmt.Errorf("download checksum: %w", err)
	}
	fields := strings.Fields(string(checksumData))
	if len(fields) == 0 || len(fields[0]) != sha256.Size*2 {
		return Result{}, errors.New("release checksum is invalid")
	}
	if err := verifyFile(archive, fields[0]); err != nil {
		return Result{}, err
	}
	staging := filepath.Join(temporary, "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		return Result{}, err
	}
	if strings.HasSuffix(info.Asset.Name, ".zip") {
		err = extractZip(archive, staging)
	} else {
		err = extractTarGz(archive, staging)
	}
	if err != nil {
		return Result{}, fmt.Errorf("extract update: %w", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return Result{}, err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return Result{}, err
	}
	name := "tapioca"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	newExecutable := filepath.Join(staging, name)
	if _, err := os.Stat(newExecutable); err != nil {
		return Result{}, fmt.Errorf("update archive does not contain %s", name)
	}
	installRoot := filepath.Dir(executable)
	if filepath.Base(executable) == name {
		if strings.Contains(filepath.ToSlash(executable), ".app/Contents/Resources/") {
			return Result{}, errors.New("this CLI belongs to Tapioca Desktop; install the update from Desktop Settings")
		}
		if runtime.GOOS == "windows" {
			persistent := filepath.Join(installRoot, ".tapioca-update-"+info.Latest)
			_ = os.RemoveAll(persistent)
			if err := os.Rename(staging, persistent); err != nil {
				return Result{}, fmt.Errorf("stage Windows update: %w", err)
			}
			script := filepath.Join(persistent, "install.cmd")
			contents := windowsInstallScript(executable, persistent)
			if err := os.WriteFile(script, []byte(contents), 0o700); err != nil {
				return Result{}, err
			}
			command := exec.Command("cmd.exe", "/c", "start", "", "/b", script)
			command.Dir = persistent
			command.Stdin, command.Stdout, command.Stderr = nil, nil, nil
			if err := command.Start(); err != nil {
				return Result{}, fmt.Errorf("start Windows updater: %w", err)
			}
			removeTemporary = true
			return Result{Info: info, Executable: executable}, nil
		}
		if err := replaceInstall(staging, installRoot, []string{"runtime", name}); err != nil {
			return Result{}, err
		}
	} else {
		return Result{}, fmt.Errorf("cannot safely identify the Tapioca installation at %s", executable)
	}
	return Result{Info: info, Executable: executable}, nil
}

func replaceInstall(staging, installRoot string, entries []string) error {
	type movedPath struct{ source, target, backup string }
	moved := make([]movedPath, 0, len(entries))
	rollback := func() {
		for index := len(moved) - 1; index >= 0; index-- {
			entry := moved[index]
			_ = os.RemoveAll(entry.target)
			if entry.backup != "" {
				_ = os.Rename(entry.backup, entry.target)
			}
		}
	}
	for _, name := range entries {
		source := filepath.Join(staging, name)
		if _, err := os.Stat(source); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			rollback()
			return err
		}
		target := filepath.Join(installRoot, name)
		backup := target + ".previous"
		_ = os.RemoveAll(backup)
		if _, err := os.Stat(target); err == nil {
			if err := os.Rename(target, backup); err != nil {
				rollback()
				return fmt.Errorf("backup %s: %w", target, err)
			}
		} else {
			backup = ""
		}
		entry := movedPath{source: source, target: target, backup: backup}
		moved = append(moved, entry)
		if err := os.Rename(source, target); err != nil {
			rollback()
			return fmt.Errorf("install %s: %w", target, err)
		}
	}
	for _, entry := range moved {
		if entry.backup != "" {
			_ = os.RemoveAll(entry.backup)
		}
	}
	return nil
}

func windowsInstallScript(executable, staging string) string {
	targetRoot := filepath.Dir(executable)
	return "@echo off\r\n" +
		"setlocal\r\n" +
		":wait\r\n" +
		"move /Y \"" + executable + "\" \"" + executable + ".previous\" >NUL 2>NUL\r\n" +
		"if errorlevel 1 (timeout /t 1 /nobreak >NUL & goto wait)\r\n" +
		"move /Y \"" + filepath.Join(staging, "tapioca.exe") + "\" \"" + executable + "\" >NUL\r\n" +
		"if exist \"" + filepath.Join(staging, "runtime") + "\" (\r\n" +
		"  if exist \"" + filepath.Join(targetRoot, "runtime.previous") + "\" rmdir /S /Q \"" + filepath.Join(targetRoot, "runtime.previous") + "\"\r\n" +
		"  if exist \"" + filepath.Join(targetRoot, "runtime") + "\" move /Y \"" + filepath.Join(targetRoot, "runtime") + "\" \"" + filepath.Join(targetRoot, "runtime.previous") + "\" >NUL\r\n" +
		"  move /Y \"" + filepath.Join(staging, "runtime") + "\" \"" + filepath.Join(targetRoot, "runtime") + "\" >NUL\r\n" +
		")\r\n" +
		"del /Q \"" + executable + ".previous\" 2>NUL\r\n" +
		"endlocal\r\n"
}

func cliAssetName(goos, goarch string) (string, error) {
	switch goos + "/" + goarch {
	case "darwin/arm64":
		return "tapioca-darwin-arm64.tar.gz", nil
	case "linux/amd64":
		return "tapioca-linux-amd64.tar.gz", nil
	case "linux/arm64":
		return "tapioca-linux-arm64.tar.gz", nil
	case "windows/amd64":
		return "tapioca-windows-amd64.zip", nil
	case "windows/arm64":
		return "tapioca-windows-arm64.zip", nil
	default:
		return "", fmt.Errorf("self-update is unsupported on %s/%s", goos, goarch)
	}
}

func validVersion(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 3 {
		return false
	}
	for _, part := range parts {
		if _, err := strconv.Atoi(part); err != nil {
			return false
		}
	}
	return true
}

func compareVersions(left, right string) int {
	a, b := strings.Split(left, "."), strings.Split(right, ".")
	for index := 0; index < 3; index++ {
		av, _ := strconv.Atoi(a[index])
		bv, _ := strconv.Atoi(b[index])
		if av < bv {
			return -1
		}
		if av > bv {
			return 1
		}
	}
	return 0
}

func download(ctx context.Context, url, destination string, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s", resp.Status)
	}
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	written, copyErr := io.Copy(file, io.LimitReader(resp.Body, limit+1))
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written > limit {
		_ = os.Remove(destination)
		return errors.New("download exceeds size limit")
	}
	return nil
}

func downloadBytes(ctx context.Context, url string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("download exceeds size limit")
	}
	return data, nil
}

func verifyFile(path, expected string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return errors.New("downloaded update checksum did not match")
	}
	return nil
}

func safeTarget(root, name string) (string, error) {
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", errors.New("archive contains an unsafe path")
	}
	return filepath.Join(root, clean), nil
}

func extractZip(path, root string) error {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, entry := range archive.File {
		target, err := safeTarget(root, entry.Name)
		if err != nil {
			return err
		}
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return errors.New("update archive contains a symlink")
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		source, err := entry.Open()
		if err != nil {
			return err
		}
		destination, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
		if err != nil {
			source.Close()
			return err
		}
		_, copyErr := io.Copy(destination, source)
		source.Close()
		destination.Close()
		if copyErr != nil {
			return copyErr
		}
	}
	return nil
}

func extractTarGz(path, root string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		target, err := safeTarget(root, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			destination, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)&0o777)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(destination, reader)
			destination.Close()
			if copyErr != nil {
				return copyErr
			}
		default:
			return errors.New("update archive contains an unsupported entry")
		}
	}
	return nil
}
