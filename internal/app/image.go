package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/imageruntime"
)

func image(args []string) error {
	return imageCommand(args, false)
}

func edit(args []string) error {
	return imageCommand(args, true)
}

func imageCommand(args []string, requireInput bool) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca image MODEL --prompt TEXT [flags]")
	}
	rawRef := args[0]
	ref, recipeAdapters, _, err := resolveComposition(rawRef, nil)
	if err != nil {
		return err
	}
	profile, err := resolveMediaModel(ref, "image")
	if err != nil {
		return err
	}
	if profile.Kind != "image" {
		return fmt.Errorf("%s is a text model; use `tapioca run %s`", profile.Name, ref)
	}
	widthDefault, heightDefault, stepsDefault := profile.Width, profile.Height, profile.Steps
	if widthDefault == 0 {
		widthDefault = 1024
	}
	if heightDefault == 0 {
		heightDefault = 1024
	}
	if stepsDefault == 0 {
		stepsDefault = 4
	}
	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	prompt := fs.String("prompt", "", "image description")
	negative := fs.String("negative-prompt", "", "content to steer away from")
	output := fs.String("output", "", "output PNG path")
	width := fs.Int("width", widthDefault, "image width (divisible by 16)")
	height := fs.Int("height", heightDefault, "image height (divisible by 16)")
	steps := fs.Int("steps", stepsDefault, "denoising steps")
	seed := fs.Uint64("seed", 0, "generation seed (default 0)")
	randomSeed := fs.Bool("random-seed", false, "generate and print a random seed")
	var inputImages stringList
	fs.Var(&inputImages, "image", "input/reference image; repeatable")
	var adapterValues stringList
	adapterValues = append(adapterValues, recipeAdapters...)
	adapterFile := ""
	var adapterScale optionalFloat
	addAdapterFlags(fs, &adapterValues, &adapterFile, &adapterScale)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *prompt == "" {
		return errors.New("usage: tapioca image MODEL --prompt TEXT [flags]")
	}
	seedSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedSet = true
		}
	})
	effectiveSeed, err := resolveMediaSeed(*seed, seedSet, *randomSeed)
	if err != nil {
		return err
	}
	if *width <= 0 || *height <= 0 || *width%16 != 0 || *height%16 != 0 {
		return errors.New("width and height must be positive and divisible by 16")
	}
	if requireInput && len(inputImages) == 0 {
		return errors.New("tapioca edit requires at least one --image")
	}
	if len(adapterValues) > 0 && profile.Backend == "mlx" {
		return errors.New(
			"the native Qwen Image Flash MLX backend cannot load LoRA adapters yet; " +
				"use a compatible MFLUX model on macOS or Diffusers model on Windows",
		)
	}
	if len(adapterValues) > 0 &&
		(profile.Backend == "onnx-directml" || profile.Backend == "onnx-cpu") {
		return errors.New(
			"ONNX diffusion models use static graphs and cannot load LoRA adapters at runtime; " +
				"use a CUDA Diffusers or macOS MFLUX model for LoRA support",
		)
	}
	if len(inputImages) > 0 &&
		(profile.Backend == "onnx-directml" || profile.Backend == "onnx-cpu") {
		return errors.New(
			"the curated ONNX diffusion backend currently supports text-to-image only; " +
				"remove --image or use CUDA Diffusers/macOS MFLUX for editing",
		)
	}
	if len(inputImages) > 0 && profile.Backend == "mlx" {
		return errors.New(
			"the native Qwen Image Flash MLX backend currently supports text-to-image only; " +
				"use flux2-klein with MFLUX for image editing",
		)
	}
	if *randomSeed {
		fmt.Fprintf(os.Stderr, "using random seed %d\n", effectiveSeed)
	}
	var explicitScale *float64
	if adapterScale.set {
		explicitScale = &adapterScale.value
	}
	adapters, err := resolveAdapters(
		adapterValues, adapterFile, explicitScale, profile.Name, profile.Backend,
	)
	if err != nil {
		return err
	}
	model, err := ensureResolvedModel(profile)
	if err != nil {
		return err
	}
	if model.Kind != "image" {
		return fmt.Errorf("%s is a text model; use `tapioca run %s`", model.Name, ref)
	}
	target := *output
	if target == "" {
		target = fmt.Sprintf("tapioca-%d.png", time.Now().Unix())
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	imagePaths := make([]string, 0, len(inputImages))
	for _, input := range inputImages {
		path, err := filepath.Abs(input)
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("input image %q: %w", input, err)
		}
		imagePaths = append(imagePaths, path)
	}
	home, err := config.Home()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	fmt.Fprintf(os.Stderr, "generating %dx%d image with %s...\n", *width, *height, model.Name)
	var guidanceScale *float64
	if profile.GuidanceScaleSet {
		guidanceScale = &profile.GuidanceScale
	}
	if err := imageruntime.Run(ctx, filepath.Join(home, "runtime"), imageruntime.Request{
		ModelPath: model.Path, Prompt: *prompt, NegativePrompt: *negative,
		Output: target, Width: *width, Height: *height, Steps: *steps, Seed: effectiveSeed,
		Backend: model.Backend, GuidanceScale: guidanceScale,
		InputImages: imagePaths, Adapters: adapters,
	}); err != nil {
		return err
	}
	fmt.Println(target)
	return nil
}
