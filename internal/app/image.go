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
	if len(args) == 0 {
		return errors.New("usage: tapioca image MODEL --prompt TEXT [flags]")
	}
	ref := args[0]
	fs := flag.NewFlagSet("image", flag.ContinueOnError)
	prompt := fs.String("prompt", "", "image description")
	negative := fs.String("negative-prompt", "", "content to steer away from")
	output := fs.String("output", "", "output PNG path")
	width := fs.Int("width", 1024, "image width (divisible by 16)")
	height := fs.Int("height", 1024, "image height (divisible by 16)")
	steps := fs.Int("steps", 4, "denoising steps")
	seed := fs.Uint64("seed", 0, "random seed")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *prompt == "" {
		return errors.New("usage: tapioca image MODEL --prompt TEXT [flags]")
	}
	if *width <= 0 || *height <= 0 || *width%16 != 0 || *height%16 != 0 {
		return errors.New("width and height must be positive and divisible by 16")
	}
	model, err := findModel(ref)
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
	home, err := config.Home()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), terminationSignals()...)
	defer stop()
	fmt.Fprintf(os.Stderr, "generating %dx%d image with %s...\n", *width, *height, model.Name)
	if err := imageruntime.Run(ctx, filepath.Join(home, "runtime"), imageruntime.Request{
		ModelPath: model.Path, Prompt: *prompt, NegativePrompt: *negative,
		Output: target, Width: *width, Height: *height, Steps: *steps, Seed: *seed,
		Backend: model.Backend,
	}); err != nil {
		return err
	}
	fmt.Println(target)
	return nil
}
