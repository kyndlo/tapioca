package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlos/tapioca/internal/catalog"
)

type hubModel struct {
	Siblings []struct {
		Filename string `json:"rfilename"`
	} `json:"siblings"`
}

func pullSnapshot(model catalog.Resolved, destination string, force bool) error {
	return pullHubSnapshot(model, destination, force, imageSnapshotFile)
}

func pullTextSnapshot(model catalog.Resolved, destination string, force bool) error {
	return pullHubSnapshot(model, destination, force, textSnapshotFile)
}

func pullHubSnapshot(
	model catalog.Resolved,
	destination string,
	force bool,
	include func(string) bool,
) error {
	resp, err := http.Get("https://huggingface.co/api/models/" + model.Repo)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Hugging Face metadata request failed: %s", resp.Status)
	}
	var metadata hubModel
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return err
	}
	var files []string
	for _, sibling := range metadata.Siblings {
		if include(sibling.Filename) {
			files = append(files, sibling.Filename)
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("%s contains no supported image-model files", model.Repo)
	}
	fmt.Printf("pulling %s snapshot from %s (%d files)\n", model.Name, model.Repo, len(files))
	for index, name := range files {
		path := filepath.Join(destination, filepath.FromSlash(name))
		if _, err := os.Stat(path); err == nil && !force {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		fmt.Printf("[%d/%d] %s\n", index+1, len(files), name)
		partial := path + ".partial"
		url := "https://huggingface.co/" + model.Repo + "/resolve/main/" + name
		if err := download(url, partial); err != nil {
			return err
		}
		if err := os.Rename(partial, path); err != nil {
			return err
		}
	}
	fmt.Printf("saved %s\n", destination)
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
	for _, prefix := range []string{"transformer/", "text_encoder/", "tokenizer/", "vae/", "scheduler/"} {
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
