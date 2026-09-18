package speechruntime

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestSoproHardwareSmoke(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("Sopro CPU candidate is currently qualified on Apple Silicon macOS only")
	}
	model := os.Getenv("TAPIOCA_SOPRO_SMOKE_MODEL")
	reference := os.Getenv("TAPIOCA_SOPRO_SMOKE_REFERENCE")
	if model == "" || reference == "" {
		t.Skip("set TAPIOCA_SOPRO_SMOKE_MODEL and TAPIOCA_SOPRO_SMOKE_REFERENCE")
	}
	output := filepath.Join(t.TempDir(), "sopro.wav")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	var stdout, stderr bytes.Buffer
	err := RunWithWriters(ctx, t.TempDir(), Request{
		ModelPath: model, ModelName: "sopro-v2-turbo:cpu-fp32",
		Text: "Hello from Tapioca. Sopro is ready to roll.", Output: output,
		VoiceSample: reference, VoiceConsent: true, Language: "en",
		Backend: "speech-sopro", Seed: 3,
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Sopro generation: %v\n%s", err, stderr.String())
	}
	var metrics struct {
		FirstAudioSeconds float64 `json:"first_audio_seconds"`
		ElapsedSeconds    float64 `json:"elapsed_seconds"`
		AudioSeconds      float64 `json:"audio_seconds"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &metrics); err != nil ||
		metrics.FirstAudioSeconds <= 0 || metrics.AudioSeconds <= 0 {
		t.Fatalf("invalid Sopro metrics %q: %v", stdout.String(), err)
	}
	t.Logf("first audio %.2fs, total %.2fs, output %.2fs", metrics.FirstAudioSeconds, metrics.ElapsedSeconds, metrics.AudioSeconds)
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) < 45 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" ||
		binary.LittleEndian.Uint16(data[22:24]) != 1 ||
		binary.LittleEndian.Uint32(data[24:28]) != 24000 ||
		binary.LittleEndian.Uint16(data[34:36]) != 16 {
		t.Fatalf("invalid Sopro WAV (%d bytes)", len(data))
	}
}
