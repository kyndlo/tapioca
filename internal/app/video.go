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

	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/videoruntime"
)

func video(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca video MODEL --prompt TEXT [flags]")
	}
	rawRef := args[0]
	ref, recipeAdapters, recipePreset, err := resolveComposition(rawRef, nil)
	if err != nil {
		return err
	}
	profile, err := resolveMediaModel(ref, "video")
	if err != nil {
		return err
	}
	if profile.Kind != "video" {
		return fmt.Errorf("%s is not a video model; run `tapioca catalog` for video models", profile.Name)
	}

	fs := flag.NewFlagSet("video", flag.ContinueOnError)
	prompt := fs.String("prompt", "", "video description")
	enhancePrompt := fs.Bool("enhance-prompt", true, "add quality and temporal-consistency guidance")
	presetDefault := "balanced"
	if recipePreset != "" {
		presetDefault = recipePreset
	}
	preset := fs.String("preset", presetDefault, "quality preset: low-memory, balanced, or quality")
	negative := fs.String("negative-prompt", "", "content to steer away from")
	inputImage := fs.String("image", "", "optional starting image")
	output := fs.String("output", "", "output MP4 path")
	width := fs.Int("width", profile.Width, "video width (divisible by 32)")
	height := fs.Int("height", profile.Height, "video height (divisible by 32)")
	frames := fs.Int("frames", profile.Frames, "number of frames (4n+1)")
	steps := fs.Int("steps", profile.Steps, "denoising steps")
	fps := fs.Int("fps", profile.FPS, "output frames per second")
	seed := fs.Uint64("seed", 0, "generation seed (default 0)")
	randomSeed := fs.Bool("random-seed", false, "generate and print a random seed")
	var adapterValues stringList
	adapterValues = append(adapterValues, recipeAdapters...)
	adapterFile := ""
	var adapterScale optionalFloat
	addAdapterFlags(fs, &adapterValues, &adapterFile, &adapterScale)
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *prompt == "" {
		return errors.New("usage: tapioca video MODEL --prompt TEXT [flags]")
	}
	changed := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { changed[f.Name] = true })
	effectiveSeed, err := resolveMediaSeed(*seed, changed["seed"], *randomSeed, os.Stderr)
	if err != nil {
		return err
	}
	defaults, err := videoPreset(profile, *preset)
	if err != nil {
		return err
	}
	if !changed["width"] {
		*width = defaults.width
	}
	if !changed["height"] {
		*height = defaults.height
	}
	if !changed["frames"] {
		*frames = defaults.frames
	}
	if !changed["steps"] {
		*steps = defaults.steps
	}
	if *width <= 0 || *height <= 0 || *width%32 != 0 || *height%32 != 0 {
		return errors.New("width and height must be positive and divisible by 32")
	}
	if profile.Backend == "comfy-h3-mps" || profile.Backend == "comfy-h3-cuda" {
		if *frames < 5 || (*frames-5)%17 != 0 {
			return errors.New("MiniMax-H3 frames must have the form 17n+5 (for example 5, 73, or 124)")
		}
	} else if *frames <= 0 || (*frames-1)%4 != 0 {
		return errors.New("frames must be positive and have the form 4n+1 (for example 17, 41, or 81)")
	}
	if profile.Name == "ltx-video:2b-fp16" && (*frames-1)%8 != 0 {
		return errors.New("LTX-Video frames must have the form 8n+1 (for example 17, 49, or 97)")
	}
	if *steps <= 0 || *fps <= 0 {
		return errors.New("steps and fps must be positive")
	}
	if profile.Backend == "mlx-video" && *fps != 24 {
		return errors.New("MLX Wan video models currently output 24 fps")
	}
	if profile.Name == "stable-video-diffusion:xt-fp16" && *inputImage == "" {
		return errors.New("stable-video-diffusion requires --image")
	}
	if profile.Name == "stable-video-diffusion:xt-fp16" && len(adapterValues) > 0 {
		return errors.New("stable-video-diffusion does not support LoRA adapters in Tapioca")
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
	target := *output
	if target == "" {
		target = fmt.Sprintf("tapioca-%d.mp4", time.Now().Unix())
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	imagePath := *inputImage
	if imagePath != "" {
		imagePath, err = filepath.Abs(imagePath)
		if err != nil {
			return err
		}
		if _, err := os.Stat(imagePath); err != nil {
			return fmt.Errorf("input image: %w", err)
		}
	}
	home, err := config.Home()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	effectivePrompt := *prompt
	if *enhancePrompt && profile.Name != "stable-video-diffusion:xt-fp16" {
		effectivePrompt = enhanceVideoPrompt(effectivePrompt)
	}
	fmt.Fprintf(os.Stderr, "generating %dx%d, %d-frame video with %s...\n",
		*width, *height, *frames, model.Name)
	err = videoruntime.Run(ctx, filepath.Join(home, "runtime"), videoruntime.Request{
		ModelPath: model.Path, Prompt: effectivePrompt, NegativePrompt: *negative,
		InputImage: imagePath, Output: target, Width: *width, Height: *height,
		Frames: *frames, Steps: *steps, FPS: *fps, Seed: effectiveSeed, Backend: model.Backend,
		Adapters: adapters,
	})
	if err != nil {
		return err
	}
	fmt.Println(target)
	return nil
}

type videoDefaults struct {
	width  int
	height int
	frames int
	steps  int
}

func videoPreset(model catalog.Resolved, preset string) (videoDefaults, error) {
	balanced := videoDefaults{model.Width, model.Height, model.Frames, model.Steps}
	switch preset {
	case "balanced":
		return balanced, nil
	case "low-memory":
		switch model.Name {
		case "wan2.2-video:5b-q8-mlx", "yume-video:5b-mlx":
			return videoDefaults{640, 352, 41, 30}, nil
		case "ltx-video:2b-fp16":
			return videoDefaults{512, 320, 17, 6}, nil
		case "minimax-h3:fl2va-int8-mac", "minimax-h3:fl2va-int8-cuda":
			return videoDefaults{640, 352, 73, 10}, nil
		default:
			return balanced, nil
		}
	case "quality":
		switch model.Name {
		case "wan2.2-video:5b-q8-mlx":
			return videoDefaults{1280, 704, 81, 40}, nil
		case "yume-video:5b-mlx":
			return videoDefaults{1280, 704, 81, 30}, nil
		case "ltx-video:2b-fp16":
			return videoDefaults{1024, 576, 97, 8}, nil
		case "minimax-h3:fl2va-int8-mac", "minimax-h3:fl2va-int8-cuda":
			return videoDefaults{768, 1376, 73, 20}, nil
		default:
			return balanced, nil
		}
	default:
		return videoDefaults{}, fmt.Errorf(
			"unknown video preset %q; use low-memory, balanced, or quality", preset,
		)
	}
}

func enhanceVideoPrompt(prompt string) string {
	return prompt + ". High detail, coherent temporal motion, consistent subject appearance, " +
		"consistent lighting, natural movement, cinematic image quality."
}
