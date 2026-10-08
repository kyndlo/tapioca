package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/carlos/tapioca/internal/audioruntime"
	"github.com/carlos/tapioca/internal/config"
)

func audio(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca audio MODEL --prompt TEXT [flags]")
	}
	profile, err := resolveMediaModel(args[0], "audio")
	if err != nil {
		return err
	}
	if profile.Kind != "audio" {
		return fmt.Errorf("%s is not an audio-generation model", profile.Name)
	}
	fs := flag.NewFlagSet("audio", flag.ContinueOnError)
	prompt := fs.String("prompt", "", "music or sound description")
	negative := fs.String("negative-prompt", "", "qualities to steer away from")
	output := fs.String("output", "", "output WAV path")
	duration := fs.Int("seconds", profile.DurationSeconds, "audio duration in seconds")
	steps := fs.Int("steps", profile.Steps, "sampling steps")
	seed := fs.Uint64("seed", 0, "generation seed (default 0)")
	randomSeed := fs.Bool("random-seed", false, "generate and print a random seed")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*prompt) == "" {
		return errors.New("usage: tapioca audio MODEL --prompt TEXT [flags]")
	}
	seedSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "seed" {
			seedSet = true
		}
	})
	effectiveSeed, err := resolveMediaSeed(*seed, seedSet, *randomSeed, os.Stderr)
	if err != nil {
		return err
	}
	if *duration <= 0 || (profile.MaxDuration > 0 && *duration > profile.MaxDuration) {
		return fmt.Errorf("seconds must be between 1 and %d for %s", profile.MaxDuration, profile.Name)
	}
	if *steps <= 0 || *steps > 200 {
		return errors.New("steps must be between 1 and 200")
	}
	model, err := ensureResolvedModel(profile)
	if err != nil {
		return err
	}
	target := *output
	if target == "" {
		target = fmt.Sprintf("tapioca-%d.wav", time.Now().Unix())
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	if strings.ToLower(filepath.Ext(target)) != ".wav" {
		return errors.New("audio generation currently outputs WAV files")
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
	fmt.Fprintf(os.Stderr, "generating %d-second audio with %s...\n", *duration, model.Name)
	if err := audioruntime.Run(ctx, filepath.Join(home, "runtime"), audioruntime.Request{
		ModelPath: model.Path, Prompt: strings.TrimSpace(*prompt),
		NegativePrompt: strings.TrimSpace(*negative), Output: target,
		Duration: *duration, Steps: *steps, Seed: effectiveSeed, Backend: model.Backend,
	}); err != nil {
		return err
	}
	fmt.Println(target)
	return nil
}
