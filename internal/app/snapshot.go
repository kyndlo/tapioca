package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlos/tapioca/internal/catalog"
)

type hubModel struct {
	PipelineTag string `json:"pipeline_tag"`
	Siblings    []struct {
		Filename string `json:"rfilename"`
	} `json:"siblings"`
}

func pullSnapshot(model catalog.Resolved, destination string, force bool) error {
	return pullSnapshotWithContext(
		context.Background(), model, destination, force, cliPullReporter,
	)
}

func pullSnapshotWithContext(
	ctx context.Context,
	model catalog.Resolved,
	destination string,
	force bool,
	report PullReporter,
) error {
	if len(model.Artifacts) > 0 {
		return pullArtifactsWithContext(ctx, model, destination, force, report)
	}
	include := imageSnapshotFile
	if model.Kind == "speech" {
		include = textSnapshotFile
	}
	if model.Repo == "stabilityai/sd-turbo" ||
		model.Repo == "stabilityai/sdxl-turbo" ||
		model.Repo == "stabilityai/stable-video-diffusion-img2vid-xt" {
		include = imageFP16SnapshotFile
	}
	if model.Kind == "video" && model.Backend == "mlx-video" {
		include = textSnapshotFile
	}
	if model.Backend == "mflux" {
		include = textSnapshotFile
	}
	if model.Gated {
		include = licensedImageSnapshotFile
	}
	return pullHubSnapshotWithContext(ctx, model, destination, force, include, report)
}

func pullArtifactsWithContext(
	ctx context.Context,
	model catalog.Resolved,
	destination string,
	force bool,
	report PullReporter,
) error {
	reportPull(report, PullProgress{
		Stage: "starting", Count: len(model.Artifacts),
		Message: fmt.Sprintf("pulling %s bundle (%d files)", model.Name, len(model.Artifacts)),
	})
	for index, artifact := range model.Artifacts {
		clean := filepath.Clean(filepath.FromSlash(artifact.Target))
		if clean == "." || filepath.IsAbs(clean) || clean == ".." ||
			strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			return fmt.Errorf("invalid artifact target %q", artifact.Target)
		}
		path := filepath.Join(destination, clean)
		if _, err := os.Stat(path); err == nil && !force {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		reportPull(report, PullProgress{
			Stage: "file", File: artifact.Target, Index: index + 1, Count: len(model.Artifacts),
			Message: fmt.Sprintf("[%d/%d] %s", index+1, len(model.Artifacts), artifact.Target),
		})
		partial := path + ".partial"
		url := "https://huggingface.co/" + artifact.Repo + "/resolve/main/" + artifact.Filename
		if err := downloadWithContext(ctx, url, partial, report); err != nil {
			return fmt.Errorf("download %s from %s: %w", artifact.Filename, artifact.Repo, err)
		}
		if err := os.Rename(partial, path); err != nil {
			return err
		}
	}
	reportPull(report, PullProgress{
		Stage: "complete", Message: "saved " + destination, Path: destination,
	})
	return nil
}

func pullTextSnapshot(model catalog.Resolved, destination string, force bool) error {
	return pullTextSnapshotWithContext(
		context.Background(), model, destination, force, cliPullReporter,
	)
}

func pullTextSnapshotWithContext(
	ctx context.Context,
	model catalog.Resolved,
	destination string,
	force bool,
	report PullReporter,
) error {
	return pullHubSnapshotWithContext(ctx, model, destination, force, textSnapshotFile, report)
}

func pullHubSnapshot(
	model catalog.Resolved,
	destination string,
	force bool,
	include func(string) bool,
) error {
	return pullHubSnapshotWithContext(
		context.Background(), model, destination, force, include, cliPullReporter,
	)
}

func pullHubSnapshotWithContext(
	ctx context.Context,
	model catalog.Resolved,
	destination string,
	force bool,
	include func(string) bool,
	report PullReporter,
) error {
	req, err := http.NewRequest(
		http.MethodGet, "https://huggingface.co/api/models/"+model.Repo, nil,
	)
	if err != nil {
		return err
	}
	token := huggingFaceToken(ctx)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req = req.WithContext(ctx)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		if model.Gated && (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) {
			return fmt.Errorf(
				"access to gated model %s was denied; accept its terms at %s and set HF_TOKEN to a Hugging Face read token",
				model.Repo, model.LicenseURL,
			)
		}
		return fmt.Errorf("Hugging Face metadata request failed: %s", resp.Status)
	}
	var metadata hubModel
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return err
	}
	if strings.HasPrefix(model.Name, "hf-") && metadata.PipelineTag != "" {
		expected := model.Kind
		if expected == "image" && !strings.Contains(metadata.PipelineTag, "image") {
			return fmt.Errorf(
				"%s is tagged %q on Hugging Face, not as an image model",
				model.Repo, metadata.PipelineTag,
			)
		}
		if expected == "video" && !strings.Contains(metadata.PipelineTag, "video") {
			return fmt.Errorf(
				"%s is tagged %q on Hugging Face, not as a video model",
				model.Repo, metadata.PipelineTag,
			)
		}
	}
	var files []string
	for _, sibling := range metadata.Siblings {
		if include(sibling.Filename) {
			files = append(files, sibling.Filename)
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("%s contains no supported model files", model.Repo)
	}
	reportPull(report, PullProgress{
		Stage:   "starting",
		Message: fmt.Sprintf("pulling %s snapshot from %s (%d files)", model.Name, model.Repo, len(files)),
		Count:   len(files),
	})
	for index, name := range files {
		path := filepath.Join(destination, filepath.FromSlash(name))
		if _, err := os.Stat(path); err == nil && !force {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		reportPull(report, PullProgress{
			Stage:   "file",
			Message: fmt.Sprintf("[%d/%d] %s", index+1, len(files), name),
			File:    name, Index: index + 1, Count: len(files),
		})
		partial := path + ".partial"
		url := "https://huggingface.co/" + model.Repo + "/resolve/main/" + name
		if err := downloadWithContext(ctx, url, partial, report); err != nil {
			if model.Gated && (strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403")) {
				return fmt.Errorf(
					"access to gated model %s was denied; accept its terms at %s and provide a valid Hugging Face read token",
					model.Repo, model.LicenseURL,
				)
			}
			return err
		}
		if err := os.Rename(partial, path); err != nil {
			return err
		}
	}
	reportPull(report, PullProgress{
		Stage: "complete", Message: "saved " + destination, Path: destination,
	})
	return nil
}

func textSnapshotFile(name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "assets/") {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".md", ".png", ".jpg", ".jpeg", ".gif":
		return false
	default:
		return true
	}
}

func imageSnapshotFile(name string) bool {
	for _, prefix := range []string{
		"transformer/", "text_encoder/", "text_encoder_2/", "tokenizer/",
		"tokenizer_2/", "feature_extractor/", "vae/", "vae_decoder/",
		"vae_encoder/", "scheduler/", "unet/",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	switch name {
	case "model_index.json", "config.json", "LICENSE", "NOTICE":
		return true
	default:
		return false
	}
}

func licensedImageSnapshotFile(name string) bool {
	if imageSnapshotFile(name) {
		return true
	}
	switch name {
	case "LICENSE.pdf", "README.md":
		return true
	default:
		return false
	}
}

func imageFP16SnapshotFile(name string) bool {
	if !imageSnapshotFile(name) {
		return false
	}
	extension := strings.ToLower(filepath.Ext(name))
	if extension == ".onnx" || strings.HasSuffix(strings.ToLower(name), ".onnx_data") {
		return false
	}
	if extension == ".safetensors" {
		return strings.Contains(strings.ToLower(name), ".fp16.")
	}
	return true
}

func pathSize(path string) (int64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}
	var size int64
	err = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}
