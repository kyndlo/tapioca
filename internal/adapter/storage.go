package adapter

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Installed struct {
	Reference string   `json:"reference"`
	Provider  Provider `json:"provider"`
	Repo      string   `json:"repo"`
	File      string   `json:"file"`
	Path      string   `json:"path"`
	Bytes     int64    `json:"bytes"`
	SHA256    string   `json:"sha256,omitempty"`
	Bases     []string `json:"bases,omitempty"`
}

type snapshot struct {
	Provider  Provider `json:"provider"`
	Repo      string   `json:"repo"`
	File      string   `json:"file"`
	Reference string   `json:"reference"`
	Revision  string   `json:"revision,omitempty"`
	SHA256    string   `json:"sha256,omitempty"`
	Bases     []string `json:"bases,omitempty"`
}

func Import(home, source, name, base string, force bool) (Local, error) {
	if strings.TrimSpace(base) == "" {
		return Local{}, errors.New("--base is required so Tapioca can validate LoRA compatibility")
	}
	absolute, err := filepath.Abs(source)
	if err != nil {
		return Local{}, err
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return Local{}, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return Local{}, errors.New("LoRA import source must be a regular file and not a symbolic link")
	}
	if !strings.EqualFold(filepath.Ext(absolute), ".safetensors") {
		return Local{}, errors.New("LoRA imports must use the .safetensors format")
	}
	if err := validateSafeTensors(absolute); err != nil {
		return Local{}, fmt.Errorf("invalid safetensors file: %w", err)
	}
	hash, err := fileSHA256(absolute)
	if err != nil {
		return Local{}, err
	}
	explicitName := strings.TrimSpace(name) != ""
	if !explicitName {
		name = slug(filepath.Base(strings.TrimSuffix(absolute, filepath.Ext(absolute))))
	}
	if !safeSegment(name) {
		return Local{}, fmt.Errorf(
			"invalid adapter name %q; use letters, numbers, dots, dashes, or underscores", name,
		)
	}
	filename := filepath.Base(absolute)
	ref := Reference{
		Provider: ProviderLocal, Repo: name, File: filename, Scale: defaultScale,
		Raw: "local://" + name + "#" + filename,
	}
	local, err := localReference(home, ref, filename, defaultScale)
	if err != nil {
		return Local{}, err
	}
	if existing, statErr := os.Lstat(local.Path); statErr == nil {
		if !existing.Mode().IsRegular() || existing.Mode()&os.ModeSymlink != 0 {
			return Local{}, errors.New("managed adapter destination is not a regular file")
		}
		existingHash, hashErr := fileSHA256(local.Path)
		if hashErr != nil {
			return Local{}, hashErr
		}
		if strings.EqualFold(existingHash, hash) {
			local.SHA256, local.Bases = hash, []string{base}
			return local, writeSnapshot(local)
		}
		if !force && explicitName {
			return Local{}, fmt.Errorf(
				"local adapter %q already exists with different weights; choose another --name or use --force", name,
			)
		}
		if !force {
			name += "-" + hash[:8]
			ref.Repo, ref.Raw = name, "local://"+name+"#"+filename
			local, err = localReference(home, ref, filename, defaultScale)
			if err != nil {
				return Local{}, err
			}
		}
	} else if !errors.Is(statErr, os.ErrNotExist) {
		return Local{}, statErr
	}
	if err := os.MkdirAll(filepath.Dir(local.Path), 0o755); err != nil {
		return Local{}, err
	}
	if filepath.Clean(absolute) != filepath.Clean(local.Path) {
		partial := local.Path + ".partial"
		_ = os.Remove(partial)
		defer os.Remove(partial)
		if err := copyRegularFile(absolute, partial); err != nil {
			return Local{}, err
		}
		if err := replaceManagedFile(partial, local.Path, force); err != nil {
			return Local{}, err
		}
	}
	local.SHA256, local.Bases = hash, []string{base}
	if err := writeSnapshot(local); err != nil {
		return Local{}, err
	}
	return local, nil
}

// replaceManagedFile preserves the old destination until a validated temporary
// file is ready. The backup path makes explicit --force replacement work on
// Windows, where os.Rename does not replace an existing file.
func replaceManagedFile(temporary, destination string, allowReplace bool) error {
	if err := os.Rename(temporary, destination); err == nil {
		return nil
	} else if !allowReplace {
		return err
	}
	if _, err := os.Lstat(destination); err != nil {
		return err
	}
	backup := destination + ".replaced"
	_ = os.Remove(backup)
	if err := os.Rename(destination, backup); err != nil {
		return err
	}
	if err := os.Rename(temporary, destination); err != nil {
		_ = os.Rename(backup, destination)
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func List(home string) ([]Installed, error) {
	root := filepath.Join(home, "adapters")
	items := make([]Installed, 0)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if errors.Is(walkErr, os.ErrNotExist) {
			return nil
		}
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".safetensors") {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		ref, file, ok := installedReference(parts)
		if !ok {
			return nil
		}
		item := Installed{
			Reference: canonicalReference(ref, file, defaultScale),
			Provider:  ref.Provider, Repo: ref.Repo, File: file,
			Path: path, Bytes: info.Size(),
		}
		if manifest, readErr := readSnapshot(adapterRoot(home, ref)); readErr == nil {
			item.SHA256, item.Bases = manifest.SHA256, manifest.Bases
		}
		items = append(items, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Reference < items[j].Reference })
	return items, nil
}

func inspectLocal(ref Reference) (Metadata, error) {
	home, err := defaultHome()
	if err != nil {
		return Metadata{}, err
	}
	root := adapterRoot(home, ref)
	manifest, manifestErr := readSnapshot(root)
	metadata := Metadata{
		Provider: ProviderLocal, Repo: ref.Repo, Type: "LORA", Files: []File{},
	}
	if manifestErr == nil {
		metadata.Revision, metadata.Bases = manifest.Revision, manifest.Bases
	}
	files := cachedAdapterFiles(home, ref)
	for _, file := range files {
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(file)))
		if err != nil {
			return Metadata{}, err
		}
		item := File{Name: file, Size: info.Size()}
		if manifestErr == nil && manifest.File == file {
			item.SHA256 = manifest.SHA256
		}
		metadata.Files = append(metadata.Files, item)
	}
	if len(metadata.Files) == 0 {
		return Metadata{}, fmt.Errorf("local adapter %q is not installed", ref.Repo)
	}
	return metadata, nil
}

func hydrateLocal(home string, ref Reference, local Local) Local {
	if manifest, err := readSnapshot(adapterRoot(home, ref)); err == nil {
		local.Revision = manifest.Revision
		local.SHA256 = manifest.SHA256
		local.Bases = manifest.Bases
	}
	return local
}

func adapterRoot(home string, ref Reference) string {
	root := filepath.Join(home, "adapters", string(ref.Provider))
	switch ref.Provider {
	case ProviderHuggingFace, ProviderModelScope, ProviderCivitai:
		left, right, _ := strings.Cut(ref.Repo, "/")
		return filepath.Join(root, left, right)
	case ProviderLocal:
		return filepath.Join(root, ref.Repo)
	default:
		return root
	}
}

func canonicalReference(ref Reference, file string, scale float64) string {
	prefix := map[Provider]string{
		ProviderHuggingFace: "hf://", ProviderCivitai: "civitai://",
		ProviderModelScope: "ms://", ProviderLocal: "local://",
	}[ref.Provider]
	value := prefix + ref.Repo
	if file != "" {
		value += "#" + filepath.ToSlash(file)
	}
	if scale != defaultScale {
		value += fmt.Sprintf("@%g", scale)
	}
	return value
}

func installedReference(parts []string) (Reference, string, bool) {
	if len(parts) < 3 {
		return Reference{}, "", false
	}
	provider := Provider(parts[0])
	var repo string
	var fileParts []string
	switch provider {
	case ProviderHuggingFace, ProviderModelScope, ProviderCivitai:
		if len(parts) < 4 || !safeSegment(parts[1]) || !safeSegment(parts[2]) {
			return Reference{}, "", false
		}
		repo, fileParts = parts[1]+"/"+parts[2], parts[3:]
	case ProviderLocal:
		if !safeSegment(parts[1]) {
			return Reference{}, "", false
		}
		repo, fileParts = parts[1], parts[2:]
	default:
		return Reference{}, "", false
	}
	file := strings.Join(fileParts, "/")
	if _, err := cleanRelativeFile(file); err != nil {
		return Reference{}, "", false
	}
	return Reference{Provider: provider, Repo: repo, Scale: defaultScale}, file, true
}

func writeSnapshot(local Local) error {
	manifest := snapshot{
		Provider: local.Provider, Repo: local.Repo, File: local.File,
		Reference: canonicalReference(
			Reference{Provider: local.Provider, Repo: local.Repo}, local.File, defaultScale,
		),
		Revision: local.Revision, SHA256: local.SHA256, Bases: local.Bases,
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Dir(local.Path), "snapshot.json"), append(data, '\n'), 0o644)
}

func readSnapshot(root string) (snapshot, error) {
	data, err := os.ReadFile(filepath.Join(root, "snapshot.json"))
	if err != nil {
		return snapshot{}, err
	}
	var manifest snapshot
	if err := json.Unmarshal(data, &manifest); err != nil {
		return snapshot{}, err
	}
	return manifest, nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", hasher.Sum(nil)), nil
}

func copyRegularFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func slug(value string) string {
	value = strings.Trim(strings.ToLower(value), " ._-")
	var builder strings.Builder
	lastDash := false
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.' || char == '_' {
			builder.WriteRune(char)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	result := strings.Trim(builder.String(), "-._")
	if result == "" {
		return "adapter"
	}
	return result
}

func defaultHome() (string, error) {
	if value := os.Getenv("TAPIOCA_HOME"); value != "" {
		return filepath.Abs(value)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tapioca"), nil
}
