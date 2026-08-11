package adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type hubModel struct {
	SHA         string `json:"sha"`
	PipelineTag string `json:"pipeline_tag"`
	CardData    struct {
		BaseModel json.RawMessage `json:"base_model"`
		License   string          `json:"license"`
	} `json:"cardData"`
	Tags     []string `json:"tags"`
	Siblings []struct {
		Filename string `json:"rfilename"`
		Size     int64  `json:"size"`
		LFS      *struct {
			SHA256 string `json:"sha256"`
			Size   int64  `json:"size"`
		} `json:"lfs"`
	} `json:"siblings"`
}

func inspectHuggingFace(client *http.Client, ref Reference) (Metadata, error) {
	req, err := http.NewRequest(http.MethodGet,
		"https://huggingface.co/api/models/"+ref.Repo+"?blobs=true", nil)
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
		Provider: ProviderHuggingFace,
		Repo:     ref.Repo, Revision: hub.SHA, Pipeline: hub.PipelineTag,
		License: hub.CardData.License, Type: typeFromTags(hub.Tags),
		Files: []File{},
	}
	metadata.Bases = decodeBaseModels(hub.CardData.BaseModel)
	for _, sibling := range hub.Siblings {
		if !strings.EqualFold(filepath.Ext(sibling.Filename), ".safetensors") {
			continue
		}
		file := File{Name: sibling.Filename, Size: sibling.Size}
		if sibling.LFS != nil {
			file.SHA256 = strings.ToLower(sibling.LFS.SHA256)
			if file.Size == 0 {
				file.Size = sibling.LFS.Size
			}
		}
		metadata.Files = append(metadata.Files, file)
	}
	return metadata, nil
}

type civitaiModel struct {
	ID         int      `json:"id"`
	Name       string   `json:"name"`
	Type       string   `json:"type"`
	BaseModels []string `json:"baseModels"`
	Versions   []struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		BaseModel string `json:"baseModel"`
		Files     []struct {
			Name        string            `json:"name"`
			SizeKB      float64           `json:"sizeKB"`
			Type        string            `json:"type"`
			Hashes      map[string]string `json:"hashes"`
			DownloadURL string            `json:"downloadUrl"`
			Metadata    struct {
				Format string `json:"format"`
			} `json:"metadata"`
		} `json:"files"`
	} `json:"modelVersions"`
}

func inspectCivitai(client *http.Client, ref Reference) (Metadata, error) {
	modelID, versionID, err := civitaiIDs(ref.Repo)
	if err != nil {
		return Metadata{}, err
	}
	endpoint, err := providerEndpoint("CIVITAI_ENDPOINT", "https://civitai.com")
	if err != nil {
		return Metadata{}, err
	}
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/api/v1/models/%d", endpoint, modelID), nil)
	if err != nil {
		return Metadata{}, err
	}
	addBearerAuth(req, "CIVITAI_TOKEN")
	resp, err := client.Do(req)
	if err != nil {
		return Metadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return Metadata{}, fmt.Errorf("Civitai metadata request failed: %s", resp.Status)
	}
	var model civitaiModel
	if err := json.NewDecoder(resp.Body).Decode(&model); err != nil {
		return Metadata{}, err
	}
	metadata := Metadata{
		Provider: ProviderCivitai,
		Repo:     ref.Repo, Revision: strconv.Itoa(versionID), Type: model.Type,
		Bases: append([]string{}, model.BaseModels...), Files: []File{},
	}
	found := false
	for _, version := range model.Versions {
		if version.ID != versionID {
			continue
		}
		found = true
		if version.BaseModel != "" {
			metadata.Bases = []string{version.BaseModel}
		}
		for _, remote := range version.Files {
			if !strings.EqualFold(filepath.Ext(remote.Name), ".safetensors") {
				continue
			}
			metadata.Files = append(metadata.Files, File{
				Name: remote.Name, Size: int64(remote.SizeKB * 1024),
				SHA256:      strings.ToLower(remote.Hashes["SHA256"]),
				DownloadURL: remote.DownloadURL,
			})
		}
		break
	}
	if !found {
		return Metadata{}, fmt.Errorf(
			"Civitai model %d does not contain version %d", modelID, versionID,
		)
	}
	return metadata, nil
}

type modelScopeEnvelope[T any] struct {
	Success bool   `json:"success"`
	Code    int    `json:"Code"`
	Message string `json:"Message"`
	Data    T      `json:"data"`
	Legacy  T      `json:"Data"`
}

type modelScopeModel struct {
	ID      string   `json:"id"`
	License string   `json:"license"`
	Tasks   []string `json:"tasks"`
	Tags    []string `json:"tags"`
	Readme  string   `json:"readme"`
	Private bool     `json:"private"`
	Gated   bool     `json:"gated"`
}

type modelScopeTree struct {
	Files []struct {
		Path     string `json:"Path"`
		Size     int64  `json:"Size"`
		SHA256   string `json:"Sha256"`
		Revision string `json:"Revision"`
		Type     string `json:"Type"`
	} `json:"Files"`
	LatestCommitter struct {
		ID string `json:"Id"`
	} `json:"LatestCommitter"`
}

func inspectModelScope(client *http.Client, ref Reference) (Metadata, error) {
	endpoint, err := providerEndpoint("MODELSCOPE_ENDPOINT", "https://modelscope.cn")
	if err != nil {
		return Metadata{}, err
	}
	modelURL := endpoint + "/openapi/v1/models/" + ref.Repo
	modelReq, err := http.NewRequest(http.MethodGet, modelURL, nil)
	if err != nil {
		return Metadata{}, err
	}
	addModelScopeAuth(modelReq)
	modelResp, err := client.Do(modelReq)
	if err != nil {
		return Metadata{}, err
	}
	defer modelResp.Body.Close()
	if modelResp.StatusCode >= 300 {
		return Metadata{}, fmt.Errorf("ModelScope metadata request failed: %s", modelResp.Status)
	}
	var modelEnvelope modelScopeEnvelope[modelScopeModel]
	if err := json.NewDecoder(modelResp.Body).Decode(&modelEnvelope); err != nil {
		return Metadata{}, err
	}
	model := modelEnvelope.Data
	if model.ID == "" {
		model = modelEnvelope.Legacy
	}

	treeURL := endpoint + "/api/v1/models/" + ref.Repo +
		"/repo/files?Revision=master&Recursive=true"
	treeReq, err := http.NewRequest(http.MethodGet, treeURL, nil)
	if err != nil {
		return Metadata{}, err
	}
	addModelScopeAuth(treeReq)
	treeResp, err := client.Do(treeReq)
	if err != nil {
		return Metadata{}, err
	}
	defer treeResp.Body.Close()
	if treeResp.StatusCode >= 300 {
		return Metadata{}, fmt.Errorf("ModelScope file listing failed: %s", treeResp.Status)
	}
	var treeEnvelope modelScopeEnvelope[modelScopeTree]
	if err := json.NewDecoder(treeResp.Body).Decode(&treeEnvelope); err != nil {
		return Metadata{}, err
	}
	tree := treeEnvelope.Legacy
	if len(tree.Files) == 0 {
		tree = treeEnvelope.Data
	}
	metadata := Metadata{
		Provider: ProviderModelScope, Repo: ref.Repo,
		Revision: tree.LatestCommitter.ID, License: model.License,
		Type: typeFromTags(model.Tags), Files: []File{},
	}
	if len(model.Tasks) > 0 {
		metadata.Pipeline = strings.Join(model.Tasks, ",")
	}
	metadata.Bases = baseModelsFromTags(model.Tags)
	for _, remote := range tree.Files {
		if !strings.EqualFold(filepath.Ext(remote.Path), ".safetensors") {
			continue
		}
		metadata.Files = append(metadata.Files, File{
			Name: remote.Path, Size: remote.Size, SHA256: strings.ToLower(remote.SHA256),
		})
		if metadata.Revision == "" {
			metadata.Revision = remote.Revision
		}
	}
	return metadata, nil
}

func decodeBaseModels(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err == nil {
		return values
	}
	var value string
	if err := json.Unmarshal(raw, &value); err == nil && value != "" {
		return []string{value}
	}
	return nil
}

func typeFromTags(tags []string) string {
	for _, tag := range tags {
		normalized := strings.ToLower(tag)
		if normalized == "lora" || strings.HasSuffix(normalized, ":lora") ||
			strings.Contains(normalized, "adapter") {
			return "LORA"
		}
	}
	return ""
}

func baseModelsFromTags(tags []string) []string {
	var bases []string
	for _, tag := range tags {
		for _, prefix := range []string{"base_model:", "base-model:"} {
			if strings.HasPrefix(strings.ToLower(tag), prefix) {
				bases = append(bases, tag[len(prefix):])
			}
		}
	}
	return bases
}

func isAdapterType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "lora", "locon", "lycoris", "dora", "adapter":
		return true
	default:
		return false
	}
}

func civitaiIDs(repo string) (int, int, error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf(
			"invalid Civitai reference %q; expected civitai://MODEL_ID/VERSION_ID", repo,
		)
	}
	modelID, modelErr := strconv.Atoi(parts[0])
	versionID, versionErr := strconv.Atoi(parts[1])
	if modelErr != nil || versionErr != nil || modelID <= 0 || versionID <= 0 {
		return 0, 0, fmt.Errorf(
			"invalid Civitai reference %q; model and version IDs must be positive numbers", repo,
		)
	}
	return modelID, versionID, nil
}

func parseCivitaiWebURL(value string) (Reference, bool) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" {
		return Reference{}, false
	}
	host := strings.ToLower(parsed.Hostname())
	if host != "civitai.com" && host != "www.civitai.com" && host != "civitai.red" && host != "www.civitai.red" {
		return Reference{}, false
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "models" {
		return Reference{}, false
	}
	modelID, err := strconv.Atoi(parts[1])
	versionID, versionErr := strconv.Atoi(parsed.Query().Get("modelVersionId"))
	if err != nil || versionErr != nil || modelID <= 0 || versionID <= 0 {
		return Reference{}, false
	}
	repo := fmt.Sprintf("%d/%d", modelID, versionID)
	return Reference{
		Raw: "civitai://" + repo, Provider: ProviderCivitai,
		Repo: repo, Scale: defaultScale,
	}, true
}

func validateRepo(provider Provider, repo string) error {
	switch provider {
	case ProviderHuggingFace, ProviderModelScope:
		if strings.Count(repo, "/") != 1 {
			return fmt.Errorf("invalid %s repository %q; expected OWNER/REPOSITORY", provider, repo)
		}
		for _, part := range strings.Split(repo, "/") {
			if !safeSegment(part) {
				return fmt.Errorf("invalid %s repository %q", provider, repo)
			}
		}
	case ProviderCivitai:
		_, _, err := civitaiIDs(repo)
		return err
	case ProviderLocal:
		if !safeSegment(repo) || strings.Contains(repo, "/") {
			return fmt.Errorf("invalid local adapter name %q", repo)
		}
	default:
		return fmt.Errorf("unsupported adapter provider %q", provider)
	}
	return nil
}

func safeSegment(value string) bool {
	if value == "" || value == "." || value == ".." {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || strings.ContainsRune("._-", char) {
			continue
		}
		return false
	}
	return true
}

func cleanRelativeFile(file string) (string, error) {
	if file == "" {
		return "", nil
	}
	clean := filepath.ToSlash(filepath.Clean(file))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") ||
		filepath.IsAbs(file) || strings.Contains(file, `\`) ||
		!strings.EqualFold(filepath.Ext(clean), ".safetensors") {
		return "", fmt.Errorf("invalid adapter file %q; expected a relative .safetensors path", file)
	}
	return clean, nil
}

func providerEndpoint(environment, fallback string) (string, error) {
	value := strings.TrimSpace(os.Getenv(environment))
	if value == "" {
		value = fallback
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" ||
		parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") {
		return "", fmt.Errorf("%s must be an HTTPS origin without a path", environment)
	}
	return strings.TrimRight(value, "/"), nil
}

func validateCivitaiDownloadURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.User != nil || parsed.Host == "" {
		return "", errors.New("Civitai returned an invalid download URL")
	}
	endpoint, err := providerEndpoint("CIVITAI_ENDPOINT", "https://civitai.com")
	if err != nil {
		return "", err
	}
	allowed, _ := url.Parse(endpoint)
	host := strings.ToLower(parsed.Hostname())
	if !strings.EqualFold(parsed.Host, allowed.Host) &&
		host != "civitai.com" && host != "www.civitai.com" &&
		host != "civitai.red" && host != "www.civitai.red" {
		return "", fmt.Errorf("Civitai returned an untrusted download host %q", parsed.Hostname())
	}
	return parsed.String(), nil
}

func addBearerAuth(req *http.Request, environment string) {
	if token := strings.TrimSpace(os.Getenv(environment)); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

func addModelScopeAuth(req *http.Request) {
	token := strings.TrimSpace(os.Getenv("MODELSCOPE_API_TOKEN"))
	if token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.AddCookie(&http.Cookie{Name: "m_session_id", Value: token, Path: "/"})
}
