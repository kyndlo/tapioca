package speechruntime

import (
	"slices"
	"testing"
)

func TestPythonArguments(t *testing.T) {
	args := pythonArguments("/runtime", Request{
		ModelPath: "/models/qwen", ModelName: "qwen3-tts:0.6b-mlx",
		Text: "hello", Output: "/tmp/hello.wav", VoiceSample: "/tmp/voice.wav",
		Transcript: "reference words", Language: "English", Backend: "speech-qwen-mlx",
	})
	for _, value := range []string{
		"--model", "/models/qwen", "--model-name", "qwen3-tts:0.6b-mlx",
		"--text", "hello", "--output", "/tmp/hello.wav", "--voice-sample",
		"/tmp/voice.wav", "--transcript", "reference words", "--language", "English",
	} {
		if !slices.Contains(args, value) {
			t.Fatalf("arguments missing %q: %#v", value, args)
		}
	}
}
