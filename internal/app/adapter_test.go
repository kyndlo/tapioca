package app

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

func TestAdapterImportAcceptsPathBeforeFlags(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	header := []byte(`{"__metadata__":{"format":"pt"}}`)
	payload := make([]byte, 8+len(header)+1)
	binary.LittleEndian.PutUint64(payload[:8], uint64(len(header)))
	copy(payload[8:], header)
	source := filepath.Join(t.TempDir(), "motion.safetensors")
	if err := os.WriteFile(source, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := adapterCommand([]string{
		"import", source, "--base", "minimax-h3", "--name", "motion",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(
		home, "adapters", "local", "motion", "motion.safetensors",
	)); err != nil {
		t.Fatalf("imported adapter missing: %v", err)
	}
}
