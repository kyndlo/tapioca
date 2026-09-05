package app

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/carlos/tapioca/internal/catalog"
	"github.com/carlos/tapioca/internal/config"
	"github.com/carlos/tapioca/internal/speechruntime"
)

type voiceProfile struct {
	Name       string    `json:"name"`
	Model      string    `json:"model"`
	Audio      string    `json:"audio"`
	Transcript string    `json:"transcript,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func tts(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca tts MODEL --text TEXT [flags]")
	}
	ref := args[0]
	profile, err := catalog.Resolve(ref)
	if err != nil {
		return err
	}
	if profile.Kind != "speech" {
		return fmt.Errorf("%s is not a speech model; run `tapioca catalog` for speech models", profile.Name)
	}
	fs := flag.NewFlagSet("tts", flag.ContinueOnError)
	text := fs.String("text", "", "text to speak")
	output := fs.String("output", "", "output WAV path")
	voice := fs.String("voice", "", "saved voice profile")
	voiceSample := fs.String("voice-sample", "", "reference voice audio")
	transcript := fs.String("transcript", "", "exact transcript of the reference audio")
	transcriptFile := fs.String("transcript-file", "", "file containing the reference transcript")
	language := fs.String("language", "", "language name or code")
	voiceConsent := fs.Bool("voice-consent", false, "confirm permission to use the reference voice")
	seed := fs.Uint64("seed", 0, "speech sampling seed (CPU backends)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || strings.TrimSpace(*text) == "" {
		return errors.New("usage: tapioca tts MODEL --text TEXT [flags]")
	}
	if *voice != "" && *voiceSample != "" {
		return errors.New("use either --voice or --voice-sample, not both")
	}
	if *transcript != "" && *transcriptFile != "" {
		return errors.New("use either --transcript or --transcript-file, not both")
	}

	sample := *voiceSample
	referenceText := *transcript
	if *voice != "" {
		saved, err := loadVoice(*voice)
		if err != nil {
			return err
		}
		sample = saved.Audio
		if referenceText == "" && *transcriptFile == "" {
			referenceText = saved.Transcript
		}
	}
	if *transcriptFile != "" {
		data, err := os.ReadFile(*transcriptFile)
		if err != nil {
			return fmt.Errorf("read transcript: %w", err)
		}
		referenceText = strings.TrimSpace(string(data))
	}
	if sample != "" {
		sample, err = filepath.Abs(sample)
		if err != nil {
			return err
		}
		if _, err := os.Stat(sample); err != nil {
			return fmt.Errorf("voice sample: %w", err)
		}
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
		return errors.New("the first speech release supports WAV output; use a .wav filename")
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
	fmt.Fprintf(os.Stderr, "generating speech with %s...\n", model.Name)
	if err := speechruntime.Run(ctx, filepath.Join(home, "runtime"), speechruntime.Request{
		ModelPath: model.Path, ModelName: model.Name, Text: *text, Output: target,
		VoiceSample: sample, Transcript: referenceText, Language: *language,
		Backend:      model.Backend,
		VoiceConsent: *voiceConsent, Seed: *seed,
	}); err != nil {
		return err
	}
	fmt.Println(target)
	return nil
}

func voiceCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca voice (create|list|inspect|remove) [NAME]")
	}
	switch args[0] {
	case "create":
		return createVoice(args[1:])
	case "list":
		return listVoices()
	case "inspect":
		if len(args) != 2 {
			return errors.New("usage: tapioca voice inspect NAME")
		}
		profile, err := loadVoice(args[1])
		if err != nil {
			return err
		}
		data, _ := json.MarshalIndent(profile, "", "  ")
		fmt.Println(string(data))
		return nil
	case "remove":
		if len(args) != 2 {
			return errors.New("usage: tapioca voice remove NAME")
		}
		dir, err := voiceDirectory(args[1])
		if err != nil {
			return err
		}
		if _, err := os.Stat(filepath.Join(dir, "voice.json")); err != nil {
			return fmt.Errorf("voice %q not found", args[1])
		}
		return os.RemoveAll(dir)
	default:
		return fmt.Errorf("unknown voice command %q", args[0])
	}
}

func createVoice(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: tapioca voice create NAME --audio FILE [flags]")
	}
	name := args[0]
	if err := validateVoiceName(name); err != nil {
		return err
	}
	fs := flag.NewFlagSet("voice create", flag.ContinueOnError)
	modelRef := fs.String("model", "chatterbox:nano", "speech model")
	audio := fs.String("audio", "", "reference voice audio")
	transcript := fs.String("transcript", "", "exact words spoken in the audio")
	transcriptFile := fs.String("transcript-file", "", "file containing the transcript")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 || *audio == "" {
		return errors.New("usage: tapioca voice create NAME --audio FILE [flags]")
	}
	resolved, err := catalog.Resolve(*modelRef)
	if err != nil {
		return err
	}
	if resolved.Kind != "speech" {
		return fmt.Errorf("%s is not a speech model", resolved.Name)
	}
	sourceAudio, err := filepath.Abs(*audio)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(sourceAudio)
	if err != nil {
		return fmt.Errorf("read voice sample: %w", err)
	}
	referenceText := strings.TrimSpace(*transcript)
	if *transcriptFile != "" {
		if *transcript != "" {
			return errors.New("use either --transcript or --transcript-file, not both")
		}
		text, err := os.ReadFile(*transcriptFile)
		if err != nil {
			return fmt.Errorf("read transcript: %w", err)
		}
		referenceText = strings.TrimSpace(string(text))
	}
	dir, err := voiceDirectory(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	extension := strings.ToLower(filepath.Ext(sourceAudio))
	if extension == "" {
		extension = ".wav"
	}
	targetAudio := filepath.Join(dir, "reference"+extension)
	if err := os.WriteFile(targetAudio, data, 0o644); err != nil {
		return err
	}
	saved := voiceProfile{
		Name: name, Model: resolved.Name, Audio: targetAudio,
		Transcript: referenceText, CreatedAt: time.Now().UTC(),
	}
	metadata, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "voice.json"), append(metadata, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("saved voice %s at %s\n", name, dir)
	return nil
}

func listVoices() error {
	root, err := voicesRoot()
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("No saved voices.")
		return nil
	}
	if err != nil {
		return err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			if _, err := os.Stat(filepath.Join(root, entry.Name(), "voice.json")); err == nil {
				names = append(names, entry.Name())
			}
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		fmt.Println("No saved voices.")
		return nil
	}
	for _, name := range names {
		profile, err := loadVoice(name)
		if err != nil {
			return err
		}
		fmt.Printf("%-24s %-28s %s\n", profile.Name, profile.Model, profile.Audio)
	}
	return nil
}

func loadVoice(name string) (voiceProfile, error) {
	dir, err := voiceDirectory(name)
	if err != nil {
		return voiceProfile{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "voice.json"))
	if errors.Is(err, os.ErrNotExist) {
		return voiceProfile{}, fmt.Errorf("voice %q not found; run `tapioca voice list`", name)
	}
	if err != nil {
		return voiceProfile{}, err
	}
	var profile voiceProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return voiceProfile{}, err
	}
	if _, err := os.Stat(profile.Audio); err != nil {
		return voiceProfile{}, fmt.Errorf("voice %q reference audio is unavailable: %w", name, err)
	}
	return profile, nil
}

func voicesRoot() (string, error) {
	home, err := config.Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "voices"), nil
}

func voiceDirectory(name string) (string, error) {
	if err := validateVoiceName(name); err != nil {
		return "", err
	}
	root, err := voicesRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

func validateVoiceName(name string) error {
	if name == "" {
		return errors.New("voice name cannot be empty")
	}
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return errors.New("voice names may contain only letters, numbers, hyphens, and underscores")
	}
	return nil
}
