package app

import (
	"errors"
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/carlos/tapioca/internal/adapter"
	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/recipe"
)

func resolveMediaModel(ref, kind string) (catalog.Resolved, error) {
	resolved, err := catalog.Resolve(ref)
	if err == nil {
		return resolved, nil
	}
	if !strings.HasPrefix(ref, "hf://") {
		return catalog.Resolved{}, err
	}
	repo := strings.TrimPrefix(ref, "hf://")
	if strings.Count(repo, "/") != 1 {
		return catalog.Resolved{}, fmt.Errorf(
			"invalid Hugging Face model %q; expected hf://OWNER/REPOSITORY", ref,
		)
	}
	for _, part := range strings.Split(repo, "/") {
		if part == "" || part == "." || part == ".." {
			return catalog.Resolved{}, fmt.Errorf("invalid Hugging Face model %q", ref)
		}
	}
	backend := "diffusers"
	width, height, steps, frames, fps := 1024, 1024, 20, 0, 0
	platform := "Windows/Linux NVIDIA"
	if kind == "video" {
		backend, width, height, steps, frames, fps = "diffusers-video", 768, 512, 25, 41, 24
		platform = "Windows x64 NVIDIA"
	}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		backend, platform = "mflux", "macOS Apple Silicon"
		if kind == "video" {
			backend = "mlx-video"
		}
	}
	name := "hf-" + strings.NewReplacer("/", "--", ":", "-").Replace(repo)
	return catalog.Resolved{
		Name: name, Repo: repo, Kind: kind, Backend: backend,
		Width: width, Height: height, Steps: steps, Frames: frames, FPS: fps,
		Size: "see Hugging Face", Memory: "model dependent",
		GPU: "backend dependent", Platform: platform,
	}, nil
}

type stringList []string

func (values *stringList) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *stringList) Set(value string) error {
	if value == "" {
		return errors.New("value cannot be empty")
	}
	*values = append(*values, value)
	return nil
}

type optionalFloat struct {
	value float64
	set   bool
}

func (value *optionalFloat) String() string {
	return fmt.Sprint(value.value)
}

func (value *optionalFloat) Set(raw string) error {
	var parsed float64
	if _, err := fmt.Sscan(raw, &parsed); err != nil {
		return err
	}
	if parsed < 0 {
		return errors.New("scale must be zero or greater")
	}
	value.value = parsed
	value.set = true
	return nil
}

func addAdapterFlags(fs *flag.FlagSet, adapters *stringList, file *string, scale *optionalFloat) {
	fs.Var(adapters, "adapter", "LoRA adapter reference; repeatable")
	fs.StringVar(file, "adapter-file", "", "weight file for a single adapter repository")
	fs.Var(scale, "adapter-scale", "strength for a single adapter")
}

func resolveComposition(ref string, commandAdapters []string) (
	string, []string, string, error,
) {
	home, err := config.Home()
	if err != nil {
		return "", nil, "", err
	}
	if !recipe.Exists(home, ref) {
		return ref, commandAdapters, "", nil
	}
	saved, err := recipe.Load(home, ref)
	if err != nil {
		return "", nil, "", err
	}
	adapters := append([]string{}, saved.Adapters...)
	adapters = append(adapters, commandAdapters...)
	return saved.Base, adapters, saved.Preset, nil
}

func createRecipe(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca create NAME --base MODEL [--adapter REFERENCE] [--preset PRESET]")
	}
	name := args[0]
	fs := flag.NewFlagSet("create", flag.ContinueOnError)
	base := fs.String("base", "", "base model")
	preset := fs.String("preset", "", "optional generation preset")
	var adapters stringList
	fs.Var(&adapters, "adapter", "LoRA adapter reference; repeatable")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *base == "" {
		return errors.New("usage: tapioca create NAME --base MODEL [--adapter REFERENCE] [--preset PRESET]")
	}
	if _, err := catalog.Resolve(*base); err != nil {
		if _, externalErr := resolveMediaModel(*base, "image"); externalErr != nil {
			return err
		}
	}
	for _, value := range adapters {
		if _, err := adapter.Parse(value); err != nil {
			return err
		}
	}
	home, err := config.Home()
	if err != nil {
		return err
	}
	if err := recipe.Save(home, recipe.Recipe{
		Name: name, Base: *base, Adapters: adapters, Preset: *preset,
	}); err != nil {
		return err
	}
	fmt.Printf("saved recipe %s\n", name)
	return nil
}
