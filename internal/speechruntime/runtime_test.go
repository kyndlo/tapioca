package speechruntime

import (
	"context"
	"io"
	"slices"
	"strings"
	"testing"
)

func TestPythonArguments(t *testing.T) {
	args := pythonArguments("/runtime", Request{
		ModelPath: "/models/qwen", ModelName: "qwen3-tts:0.6b-mlx",
		Text: "hello", Output: "/tmp/hello.wav", VoiceSample: "/tmp/voice.wav",
		Transcript: "reference words", Language: "English", Backend: "speech-qwen-mlx",
		VoiceConsent: true, Seed: 42,
	})
	for _, value := range []string{
		"--model", "/models/qwen", "--model-name", "qwen3-tts:0.6b-mlx",
		"--text", "hello", "--output", "/tmp/hello.wav", "--voice-sample",
		"/tmp/voice.wav", "--transcript", "reference words", "--language", "English",
		"--voice-consent", "--seed", "42",
	} {
		if !slices.Contains(args, value) {
			t.Fatalf("arguments missing %q: %#v", value, args)
		}
	}
}

func TestCPUVoiceConsentBeforeSetup(t *testing.T) {
	for _, backend := range []string{"speech-audio8-onnx", "speech-pocket-tts"} {
		err := RunWithWriters(context.Background(), t.TempDir(), Request{Backend: backend, VoiceSample: "reference.wav"}, io.Discard, io.Discard)
		if err == nil || !(strings.Contains(err.Error(), "permission") || strings.Contains(err.Error(), "requires x64")) {
			t.Fatalf("%s did not reject before setup: %v", backend, err)
		}
	}
}
