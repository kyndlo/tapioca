package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/carlos/tapioca/internal/catalog"
)

const VisionRuntimeVersion = "b11696"
const VisionRuntimeCommit = "ceefd16da"

func (s *Server) validateVisionModel() error {
	m := s.opts.Model
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return fmt.Errorf("MiniCPM vision is only qualified on macOS Apple Silicon")
	}
	if s.opts.Context < 1 || s.opts.Context > 8192 {
		return fmt.Errorf("vision context must be between 1 and 8192 tokens")
	}
	if strings.HasPrefix(m.Name, "minicpm-v-4.7:") &&
		(m.Integrity.SHA256 != "588c6c94c82f756b86dba35c42ed9ba9ec187d65824eb52f039c347df563e63d" ||
			m.ProjectorIntegrity.SHA256 != "fb9cc047a3cdad5317fd8433a4aeda2d22f00f02ca4bd93b34ad84165b25a900") {
		return fmt.Errorf("MiniCPM 1B requires its pinned model and matching 1B projector; pull it again")
	}
	if m.ProjectorPath == "" {
		return fmt.Errorf("missing vision projector; pull %s again", m.Name)
	}
	for _, file := range []struct {
		path     string
		metadata catalog.Download
	}{
		{m.Path, m.Integrity}, {m.ProjectorPath, m.ProjectorIntegrity},
	} {
		if file.metadata.Revision == "" || file.metadata.SHA256 == "" || file.metadata.SizeBytes <= 0 {
			return fmt.Errorf("missing model/projector integrity metadata; pull %s again", m.Name)
		}
		if err := catalog.VerifyArtifact(file.path, file.metadata); err != nil {
			return fmt.Errorf("vision artifact %s: %w", file.path, err)
		}
	}
	if m.Integrity.Revision != m.ProjectorIntegrity.Revision {
		return fmt.Errorf("model/projector revision mismatch")
	}
	return nil
}

func visionRuntimePath() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(executable), "runtime", "llama-vision-"+VisionRuntimeVersion, "llama-server")
}

func verifyVisionRuntime(ctx context.Context, path string) error {
	output, err := exec.CommandContext(ctx, path, "--version").CombinedOutput()
	if err != nil {
		return fmt.Errorf("vision runtime %s is required; run scripts/prepare-vision-runtime.sh: %w", VisionRuntimeVersion, err)
	}
	if !validVisionRuntimeVersion(string(output)) {
		return fmt.Errorf("vision requires pinned llama.cpp %s (%s); refusing stale or unqualified runtime: %s", VisionRuntimeVersion, VisionRuntimeCommit, strings.TrimSpace(string(output)))
	}
	return nil
}

func (s *Server) llamaArgs() []string {
	layers := "999"
	if s.opts.CPU {
		layers = "0"
	}
	args := []string{
		"--model", s.opts.Model.Path,
		"--host", "127.0.0.1",
		"--port", fmt.Sprint(s.opts.UpstreamPort),
		"--ctx-size", fmt.Sprint(s.opts.Context),
		"--jinja", "--n-gpu-layers", layers,
	}
	if s.opts.Model.Backend == "llama-vision" {
		args = append(args, "--mmproj", s.opts.Model.ProjectorPath, "--parallel", "1", "--chat-template-kwargs", `{"enable_thinking":false}`, "--n-predict", "512")
		if s.opts.CPU {
			args = append(args, "--no-mmproj-offload")
		}
	}
	if !s.opts.Verbose {
		args = append(args, "-lv", "0")
	}
	return args
}

func validVisionRuntimeVersion(output string) bool {
	return strings.Contains(output, "build 11696, commit "+VisionRuntimeCommit)
}
