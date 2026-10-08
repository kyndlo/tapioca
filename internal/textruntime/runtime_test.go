package textruntime

import (
	"strings"
	"testing"
)

func TestMLXVLMRuntimeIsCurrent(t *testing.T) {
	requirements, err := source.ReadFile("requirements.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(requirements), "1dcc142dbe2c579d1de7638aaac263f7e4e11734.zip") {
		t.Fatalf("MLX-VLM runtime is not pinned to v0.7.6: %s", requirements)
	}
}
