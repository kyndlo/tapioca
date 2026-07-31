package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultScale = 1.0

type Reference struct {
	Raw   string
	Repo  string
	File  string
	Scale float64
}

type File struct {
	Name string
	Size int64
}

type Metadata struct {
	Repo     string
	Revision string
	Pipeline string
	License  string
	Bases    []string
	Files    []File
}

type Local struct {
	Reference string  `json:"reference"`
	Repo      string  `json:"repo"`
	File      string  `json:"file"`
	Path      string  `json:"path"`
	Scale     float64 `json:"scale"`
}

type hubModel struct {
	SHA         string `json:"sha"`
	PipelineTag string `json:"pipeline_tag"`
	CardData    struct {
		BaseModel json.RawMessage `json:"base_model"`
		License   string          `json:"license"`
	} `json:"cardData"`
	Siblings []struct {
		Filename string `json:"rfilename"`
		Size     int64  `json:"size"`
	} `json:"siblings"`
}

func Parse(value string) (Reference, error) {
	const prefix = "hf://"
	if !strings.HasPrefix(value, prefix) {
		return Reference{}, fmt.Errorf("adapter %q must start with hf://", value)
	}
	raw := value
	value = strings.TrimPrefix(value, prefix)
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
	if strings.Count(repo, "/") != 1 {
		return Reference{}, fmt.Errorf(
			"invalid Hugging Face adapter repository %q; expected hf://OWNER/REPOSITORY", repo,
		)
	}
	for _, part := range strings.Split(repo, "/") {
		if part == "" || part == "." || part == ".." {
			return Reference{}, fmt.Errorf("invalid Hugging Face adapter repository %q", repo)
		}
	}
	if file != "" {
		clean := filepath.ToSlash(filepath.Clean(file))
		if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") ||
			filepath.IsAbs(file) {
			return Reference{}, fmt.Errorf("invalid adapter file %q", file)
		}
		file = clean
	}
	return Reference{Raw: raw, Repo: repo, File: file, Scale: scale}, nil
}

func Inspect(client *http.Client, repo string) (Metadata, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequest(http.MethodGet,
		"https://huggingface.co/api/models/"+repo+"?blobs=true", nil)
	if err != nil {
		return Metadata{}, err
	}
	addHuggingFaceAuth(req)
	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Metadata{}, fmt.Errorf("Hugging Face metadata request failed: %s", resp.Status)
	}
	var hub hubModel
	if err := json.NewDecoder(resp.Body).Decode(&hub); err != nil {
		return Metadata{}, err
	}
	metadata := Metadata{
		Repo: repo, Revision: hub.SHA, Pipeline: hub.PipelineTag,
		License: hub.CardData.License,
	}
	if len(hub.CardData.BaseModel) > 0 && string(hub.CardData.BaseModel) != "null" {
		if err := json.Unmarshal(hub.CardData.BaseModel, &metadata.Bases); err != nil {
			var base string
			if stringErr := json.Unmarshal(hub.CardData.BaseModel, &base); stringErr == nil {
				metadata.Bases = []string{base}
			}
		}
	}
	for _, sibling := range hub.Siblings {
		if strings.EqualFold(filepath.Ext(sibling.Filename), ".safetensors") {
			metadata.Files = append(metadata.Files, File{
				Name: sibling.Filename,
				Size: sibling.Size,
			})
		}
	}
	return metadata, nil
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
			return local, nil
		}
	} else if cached := cachedAdapterFiles(home, ref.Repo); len(cached) == 1 {
		return localReference(home, ref, cached[0], scale)
	}
	metadata, err := Inspect(client, ref.Repo)
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
	return localReference(home, ref, file, scale)
}

func cachedAdapterFiles(home, repo string) []string {
	owner, name, _ := strings.Cut(repo, "/")
	root := filepath.Join(home, "adapters", "huggingface", owner, name)
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
	clean := filepath.ToSlash(filepath.Clean(file))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") ||
		filepath.IsAbs(file) {
		return Local{}, fmt.Errorf("invalid adapter file %q", file)
	}
	owner, name, _ := strings.Cut(ref.Repo, "/")
	path := filepath.Join(home, "adapters", "huggingface", owner, name, filepath.FromSlash(clean))
	return Local{
		Reference: ref.Raw,
		Repo:      ref.Repo,
		File:      clean,
		Path:      path,
		Scale:     scale,
	}, nil
}

func Pull(client *http.Client, local Local, force bool) error {
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
	req, err := http.NewRequest(http.MethodGet,
		"https://huggingface.co/"+local.Repo+"/resolve/main/"+local.File, nil)
	if err != nil {
		return err
	}
	addHuggingFaceAuth(req)
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
	_, copyErr := io.Copy(out, resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(partial, local.Path); err != nil {
		return err
	}
	metadata := map[string]any{
		"repo":      local.Repo,
		"file":      local.File,
		"reference": local.Reference,
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
	if backend == "mlx" {
		return fmt.Errorf("backend %s cannot load LoRA adapters", backend)
	}
	return nil
}

func modelFamily(value string) string {
	for _, family := range []string{"flux", "qwen", "wan", "ltx"} {
		if strings.Contains(value, family) {
			return family
		}
	}
	return ""
}
