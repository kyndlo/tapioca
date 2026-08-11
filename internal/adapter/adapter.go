package adapter

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultScale = 1.0

type Provider string

const (
	ProviderHuggingFace Provider = "huggingface"
	ProviderCivitai     Provider = "civitai"
	ProviderModelScope  Provider = "modelscope"
	ProviderLocal       Provider = "local"
)

type Reference struct {
	Raw      string
	Provider Provider
	Repo     string
	File     string
	Scale    float64
}

type File struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	SHA256      string `json:"sha256,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
}

type Metadata struct {
	Provider Provider `json:"provider"`
	Repo     string   `json:"repo"`
	Revision string   `json:"revision,omitempty"`
	Pipeline string   `json:"pipeline,omitempty"`
	Type     string   `json:"type,omitempty"`
	License  string   `json:"license,omitempty"`
	Bases    []string `json:"bases,omitempty"`
	Files    []File   `json:"files"`
}

type Local struct {
	Reference   string   `json:"reference"`
	Provider    Provider `json:"provider"`
	Repo        string   `json:"repo"`
	File        string   `json:"file"`
	Path        string   `json:"path"`
	Scale       float64  `json:"scale"`
	SHA256      string   `json:"sha256,omitempty"`
	DownloadURL string   `json:"download_url,omitempty"`
	Revision    string   `json:"revision,omitempty"`
	Bases       []string `json:"bases,omitempty"`
}

func Parse(value string) (Reference, error) {
	raw := value
	if parsed, ok := parseCivitaiWebURL(value); ok {
		return parsed, nil
	}
	provider, body, ok := strings.Cut(value, "://")
	if !ok {
		return Reference{}, fmt.Errorf(
			"adapter %q must start with hf://, civitai://, ms://, modelscope://, or local://", value,
		)
	}
	var kind Provider
	switch strings.ToLower(provider) {
	case "hf":
		kind = ProviderHuggingFace
	case "civitai":
		kind = ProviderCivitai
	case "ms", "modelscope":
		kind = ProviderModelScope
	case "local":
		kind = ProviderLocal
	default:
		return Reference{}, fmt.Errorf("unsupported adapter provider %q", provider)
	}
	value = body
	scale := defaultScale
	if at := strings.LastIndex(value, "@"); at >= 0 {
		parsed, err := strconv.ParseFloat(value[at+1:], 64)
		if err != nil {
			return Reference{}, fmt.Errorf("invalid adapter scale %q", value[at+1:])
		}
		if parsed < 0 {
			return Reference{}, errors.New("adapter scale must be zero or greater")
		}
		scale = parsed
		value = value[:at]
	}
	repo, file, _ := strings.Cut(value, "#")
	if err := validateRepo(kind, repo); err != nil {
		return Reference{}, err
	}
	cleanFile, err := cleanRelativeFile(file)
	if err != nil {
		return Reference{}, err
	}
	return Reference{
		Raw: raw, Provider: kind, Repo: repo, File: cleanFile, Scale: scale,
	}, nil
}

func Inspect(client *http.Client, ref Reference) (Metadata, error) {
	if client == nil {
		client = http.DefaultClient
	}
	switch ref.Provider {
	case ProviderHuggingFace:
		return inspectHuggingFace(client, ref)
	case ProviderCivitai:
		return inspectCivitai(client, ref)
	case ProviderModelScope:
		return inspectModelScope(client, ref)
	case ProviderLocal:
		return inspectLocal(ref)
	default:
		return Metadata{}, fmt.Errorf("unsupported adapter provider %q", ref.Provider)
	}
}

func Resolve(client *http.Client, home string, ref Reference, explicitFile string, explicitScale *float64) (Local, error) {
	file := ref.File
	if explicitFile != "" {
		if file != "" && file != explicitFile {
			return Local{}, errors.New("adapter file was specified in both the reference and --adapter-file")
		}
		file = explicitFile
	}
	scale := ref.Scale
	if explicitScale != nil {
		scale = *explicitScale
	}
	if file != "" {
		local, err := localReference(home, ref, file, scale)
		if err != nil {
			return Local{}, err
		}
		if _, err := os.Stat(local.Path); err == nil {
			return hydrateLocal(home, ref, local), nil
		}
	} else if cached := cachedAdapterFiles(home, ref); len(cached) == 1 {
		local, err := localReference(home, ref, cached[0], scale)
		if err != nil {
			return Local{}, err
		}
		return hydrateLocal(home, ref, local), nil
	}
	metadata, err := Inspect(client, ref)
	if err != nil {
		return Local{}, err
	}
	if file == "" {
		if len(metadata.Files) == 0 {
			return Local{}, fmt.Errorf("%s contains no .safetensors adapter files", ref.Repo)
		}
		if len(metadata.Files) > 1 {
			return Local{}, fmt.Errorf(
				"%s contains %d adapter files; select one with #FILE or --adapter-file",
				ref.Repo, len(metadata.Files),
			)
		}
		file = metadata.Files[0].Name
	}
	found := false
	for _, candidate := range metadata.Files {
		if candidate.Name == file {
			found = true
			break
		}
	}
	if !found {
		return Local{}, fmt.Errorf("adapter file %q was not found in %s", file, ref.Repo)
	}
	local, err := localReference(home, ref, file, scale)
	if err != nil {
		return Local{}, err
	}
	local.Revision = metadata.Revision
	local.Bases = metadata.Bases
	for _, candidate := range metadata.Files {
		if candidate.Name == file {
			local.SHA256 = candidate.SHA256
			local.DownloadURL = candidate.DownloadURL
			break
		}
	}
	if ref.Provider == ProviderCivitai && !isAdapterType(metadata.Type) {
		return Local{}, fmt.Errorf(
			"Civitai model %s is type %q, not a LoRA adapter", ref.Repo, metadata.Type,
		)
	}
	return local, nil
}

func cachedAdapterFiles(home string, ref Reference) []string {
	root := adapterRoot(home, ref)
	var files []string
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".safetensors") {
			if relative, relativeErr := filepath.Rel(root, path); relativeErr == nil {
				files = append(files, filepath.ToSlash(relative))
			}
		}
		return nil
	})
	return files
}

func localReference(home string, ref Reference, file string, scale float64) (Local, error) {
	clean, err := cleanRelativeFile(file)
	if err != nil {
		return Local{}, err
	}
	path := filepath.Join(adapterRoot(home, ref), filepath.FromSlash(clean))
	return Local{
		Reference: canonicalReference(ref, clean, scale),
		Provider:  ref.Provider,
		Repo:      ref.Repo,
		File:      clean,
		Path:      path,
		Scale:     scale,
	}, nil
}

func Pull(client *http.Client, local Local, force bool) error {
	if local.Provider == ProviderLocal {
		if _, err := os.Stat(local.Path); err != nil {
			return fmt.Errorf("local adapter is not installed: %w", err)
		}
		return nil
	}
	if !force {
		if _, err := os.Stat(local.Path); err == nil {
			return nil
		}
	}
	if client == nil {
		client = http.DefaultClient
	}
	if err := os.MkdirAll(filepath.Dir(local.Path), 0o755); err != nil {
		return err
	}
	partial := local.Path + ".partial"
	_ = os.Remove(partial)
	defer os.Remove(partial)
	downloadURL, err := adapterDownloadURL(local)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, downloadURL, nil)
	if err != nil {
		return err
	}
	switch local.Provider {
	case ProviderHuggingFace:
		addHuggingFaceAuth(req)
	case ProviderCivitai:
		addBearerAuth(req, "CIVITAI_TOKEN")
	case ProviderModelScope:
		addModelScopeAuth(req)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("adapter download failed: %s", resp.Status)
	}
	out, err := os.OpenFile(partial, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hasher), resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if local.SHA256 != "" && !strings.EqualFold(local.SHA256, actualHash) {
		return fmt.Errorf(
			"adapter checksum mismatch: expected %s, received %s", local.SHA256, actualHash,
		)
	}
	if resp.ContentLength >= 0 && written != resp.ContentLength {
		return fmt.Errorf(
			"adapter download was incomplete: expected %d bytes, received %d",
			resp.ContentLength, written,
		)
	}
	if err := validateSafeTensors(partial); err != nil {
		return fmt.Errorf("downloaded adapter is not a valid safetensors file: %w", err)
	}
	if err := replaceManagedFile(partial, local.Path, force); err != nil {
		return err
	}
	metadata := struct {
		Provider  Provider `json:"provider"`
		Repo      string   `json:"repo"`
		File      string   `json:"file"`
		Reference string   `json:"reference"`
		Revision  string   `json:"revision,omitempty"`
		SHA256    string   `json:"sha256"`
		Bases     []string `json:"bases,omitempty"`
	}{
		Provider: local.Provider, Repo: local.Repo, File: local.File,
		Reference: local.Reference, Revision: local.Revision,
		SHA256: actualHash, Bases: local.Bases,
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	repositoryRoot := local.Path
	for range strings.Split(filepath.ToSlash(local.File), "/") {
		repositoryRoot = filepath.Dir(repositoryRoot)
	}
	return os.WriteFile(filepath.Join(repositoryRoot, "snapshot.json"),
		append(data, '\n'), 0o644)
}

func adapterDownloadURL(local Local) (string, error) {
	switch local.Provider {
	case ProviderHuggingFace:
		revision := local.Revision
		if revision == "" {
			revision = "main"
		}
		return "https://huggingface.co/" + local.Repo + "/resolve/" +
			url.PathEscape(revision) + "/" + strings.ReplaceAll(local.File, "#", "%23"), nil
	case ProviderCivitai:
		if local.DownloadURL == "" {
			_, versionID, err := civitaiIDs(local.Repo)
			if err != nil {
				return "", err
			}
			endpoint, err := providerEndpoint("CIVITAI_ENDPOINT", "https://civitai.com")
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("%s/api/download/models/%d", endpoint, versionID), nil
		}
		return validateCivitaiDownloadURL(local.DownloadURL)
	case ProviderModelScope:
		endpoint, err := providerEndpoint("MODELSCOPE_ENDPOINT", "https://modelscope.cn")
		if err != nil {
			return "", err
		}
		revision := local.Revision
		if revision == "" {
			revision = "master"
		}
		values := url.Values{"Revision": {revision}, "FilePath": {local.File}}
		return endpoint + "/api/v1/models/" + local.Repo + "/repo?" + values.Encode(), nil
	default:
		return "", fmt.Errorf("provider %q does not support downloads", local.Provider)
	}
}

func validateSafeTensors(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() < 10 {
		return errors.New("file is too small")
	}
	var prefix [8]byte
	if _, err := io.ReadFull(file, prefix[:]); err != nil {
		return err
	}
	headerSize := binary.LittleEndian.Uint64(prefix[:])
	if headerSize < 2 || headerSize > 100*1024*1024 || headerSize > uint64(info.Size()-8) {
		return fmt.Errorf("invalid header size %d", headerSize)
	}
	header := make([]byte, int(headerSize))
	if _, err := io.ReadFull(file, header); err != nil {
		return err
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(header, &decoded); err != nil {
		return fmt.Errorf("invalid JSON header: %w", err)
	}
	return nil
}

func addHuggingFaceAuth(req *http.Request) {
	token := os.Getenv("HF_TOKEN")
	if token == "" {
		token = os.Getenv("HUGGING_FACE_HUB_TOKEN")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func ValidateCompatibility(baseName, backend string, local Local) error {
	base := strings.ToLower(baseName)
	file := strings.ToLower(local.File)
	fileFamily, baseFamily := modelFamily(file), modelFamily(base)
	if fileFamily != "" && baseFamily != "" && fileFamily != baseFamily {
		return fmt.Errorf(
			"adapter file %s appears to target %s but base model %s appears to be %s",
			local.File, fileFamily, baseName, baseFamily,
		)
	}
	for _, declared := range local.Bases {
		declaredFamily := modelFamily(strings.ToLower(declared))
		if declaredFamily != "" && baseFamily != "" && declaredFamily != baseFamily {
			return fmt.Errorf(
				"adapter %s declares base model %s but selected model %s appears to be %s",
				local.Reference, declared, baseName, baseFamily,
			)
		}
	}
	if backend == "mlx" {
		return fmt.Errorf("backend %s cannot load LoRA adapters", backend)
	}
	return nil
}

func modelFamily(value string) string {
	for _, family := range []string{"minimax-h3", "minimax_h3", "flux", "qwen", "wan", "ltx"} {
		if strings.Contains(value, family) {
			if strings.HasPrefix(family, "minimax") {
				return "minimax-h3"
			}
			return family
		}
	}
	return ""
}
