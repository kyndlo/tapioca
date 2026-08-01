package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVoiceNameValidation(t *testing.T) {
	for _, name := range []string{"carlos", "narrator-1", "voice_test"} {
		if err := validateVoiceName(name); err != nil {
			t.Errorf("%q should be valid: %v", name, err)
		}
	}
	for _, name := range []string{"", "../voice", "two words", "voice/name"} {
		if err := validateVoiceName(name); err == nil {
			t.Errorf("%q should be invalid", name)
		}
	}
}

func TestCreateAndLoadVoice(t *testing.T) {
	home := filepath.Join(t.TempDir(), "tapioca")
	t.Setenv("TAPIOCA_HOME", home)
	audio := filepath.Join(t.TempDir(), "sample.wav")
	if err := os.WriteFile(audio, []byte("test audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := createVoice([]string{
		"narrator", "--model", "chatterbox:nano", "--audio", audio,
		"--transcript", "hello there",
	}); err != nil {
		t.Fatal(err)
	}
	profile, err := loadVoice("narrator")
	if err != nil {
		t.Fatal(err)
	}
	if profile.Model != "chatterbox:nano" || profile.Transcript != "hello there" {
		t.Fatalf("unexpected voice profile: %#v", profile)
	}
}
