package app

import (
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/carlos/tapioca/internal/adapter"
	"github.com/carlos/tapioca/internal/config"
)

func adapterCommand(args []string) error {
	if len(args) == 1 && args[0] == "list" {
		home, err := config.Home()
		if err != nil {
			return err
		}
		root := filepath.Join(home, "adapters", "huggingface")
		var found bool
		err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if errors.Is(walkErr, os.ErrNotExist) {
				return nil
			}
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.EqualFold(filepath.Ext(path), ".safetensors") {
				return nil
			}
			if !found {
				fmt.Println("ADAPTER\tPATH")
				found = true
			}
			relative, _ := filepath.Rel(root, path)
			fmt.Printf("%s\t%s\n", filepath.ToSlash(relative), path)
			return nil
		})
		if err != nil {
			return err
		}
		if !found {
			fmt.Println("No adapters installed.")
		}
		return nil
	}
	if len(args) < 2 {
		return errors.New("usage: tapioca adapter (inspect|pull|list) [REFERENCE] [flags]")
	}
	action, value := args[0], args[1]
	ref, err := adapter.Parse(value)
	if err != nil {
		return err
	}
	switch action {
	case "inspect":
		if len(args) != 2 {
			return errors.New("usage: tapioca adapter inspect hf://OWNER/REPOSITORY")
		}
		metadata, err := adapter.Inspect(http.DefaultClient, ref.Repo)
		if err != nil {
			return err
		}
		fmt.Printf("REPOSITORY\t%s\n", metadata.Repo)
		if metadata.Revision != "" {
			fmt.Printf("REVISION\t%s\n", metadata.Revision)
		}
		if metadata.Pipeline != "" {
			fmt.Printf("TASK\t%s\n", metadata.Pipeline)
		}
		if metadata.License != "" {
			fmt.Printf("LICENSE\t%s\n", metadata.License)
		}
		for _, base := range metadata.Bases {
			fmt.Printf("BASE MODEL\t%s\n", base)
		}
		if len(metadata.Files) == 0 {
			fmt.Println("FILES\t(no .safetensors files)")
			return nil
		}
		fmt.Println("FILE\tSIZE")
		for _, file := range metadata.Files {
			fmt.Printf("%s\t%s\n", file.Name, humanBytes(file.Size))
		}
		return nil
	case "pull":
		fs := flag.NewFlagSet("adapter pull", flag.ContinueOnError)
		file := fs.String("file", "", "exact .safetensors file in the repository")
		force := fs.Bool("force", false, "download again")
		if err := fs.Parse(args[2:]); err != nil {
			return err
		}
		if fs.NArg() != 0 {
			return errors.New("usage: tapioca adapter pull REFERENCE [--file FILE] [--force]")
		}
		home, err := config.Home()
		if err != nil {
			return err
		}
		local, err := adapter.Resolve(http.DefaultClient, home, ref, *file, nil)
		if err != nil {
			return err
		}
		fmt.Printf("pulling %s#%s\n", local.Repo, local.File)
		if err := adapter.Pull(http.DefaultClient, local, *force); err != nil {
			return err
		}
		fmt.Printf("saved %s\n", local.Path)
		return nil
	default:
		return fmt.Errorf("unknown adapter command %q; use inspect or pull", action)
	}
}

func humanBytes(size int64) string {
	if size <= 0 {
		return "unknown"
	}
	const gib = 1024 * 1024 * 1024
	const mib = 1024 * 1024
	if size >= gib {
		return fmt.Sprintf("%.1f GiB", float64(size)/gib)
	}
	return fmt.Sprintf("%.1f MiB", float64(size)/mib)
}

func resolveAdapters(
	values []string,
	explicitFile string,
	explicitScale *float64,
	baseName string,
	backend string,
) ([]adapter.Local, error) {
	if len(values) == 0 {
		if explicitFile != "" || explicitScale != nil {
			return nil, errors.New("--adapter-file and --adapter-scale require --adapter")
		}
		return nil, nil
	}
	if len(values) > 1 && (explicitFile != "" || explicitScale != nil) {
		return nil, errors.New(
			"--adapter-file and --adapter-scale can only be used with one --adapter; " +
				"use #FILE and @SCALE in repeated adapter references",
		)
	}
	home, err := config.Home()
	if err != nil {
		return nil, err
	}
	locals := make([]adapter.Local, 0, len(values))
	for _, value := range values {
		ref, err := adapter.Parse(value)
		if err != nil {
			return nil, err
		}
		local, err := adapter.Resolve(http.DefaultClient, home, ref, explicitFile, explicitScale)
		if err != nil {
			return nil, err
		}
		if err := adapter.ValidateCompatibility(baseName, backend, local); err != nil {
			return nil, err
		}
		if _, err := os.Stat(local.Path); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return nil, err
			}
			fmt.Printf("adapter %s#%s is not installed; pulling it now\n", local.Repo, local.File)
			if err := adapter.Pull(http.DefaultClient, local, false); err != nil {
				return nil, err
			}
		}
		locals = append(locals, local)
	}
	return locals, nil
}

func resolveModelLoRAs(
	values []string,
	explicitScale *float64,
	modelPath, baseName, backend string,
) ([]adapter.Local, error) {
	if len(values) == 0 {
		if explicitScale != nil {
			return nil, errors.New("--lora-scale requires --lora")
		}
		return nil, nil
	}
	if len(values) > 1 && explicitScale != nil {
		return nil, errors.New(
			"--lora-scale can only be used with one --lora; use @SCALE with repeated --lora flags",
		)
	}
	locals := make([]adapter.Local, 0, len(values))
	for _, value := range values {
		local, err := adapter.ResolveModelLoRA(modelPath, value, explicitScale)
		if err != nil {
			return nil, err
		}
		if err := adapter.ValidateCompatibility(baseName, backend, local); err != nil {
			return nil, err
		}
		locals = append(locals, local)
	}
	return locals, nil
}
