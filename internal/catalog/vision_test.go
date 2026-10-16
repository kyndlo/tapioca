package catalog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMiniCPMCandidate(t *testing.T) {
	data, err := os.ReadFile("../../catalog/candidates/minicpm-v-4.7.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateManifest(data); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "catalog.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	defer SetOverridePathForTest(path)()
	model, err := ResolveForPlatform("minicpm-v-4.7", "darwin", "arm64")
	if err != nil {
		t.Fatal(err)
	}
	if model.Projector == nil || model.Download.SizeBytes+model.Projector.SizeBytes != 1782121664 ||
		!strings.Contains(model.URL, "/6c2f4cb8bde51a00247e11003bbf66ca7c4f9249/") || model.License != "Apache-2.0" || model.Gated || DefaultContext(model.Name) != 4096 {
		t.Fatalf("incorrect vision metadata: %#v", model)
	}
	for _, platform := range [][2]string{{"linux", "amd64"}, {"windows", "amd64"}, {"windows", "arm64"}, {"darwin", "amd64"}} {
		if _, err := ResolveForPlatform(model.Name, platform[0], platform[1]); err == nil {
			t.Fatalf("advertised unqualified platform %v", platform)
		}
	}
	manifest, err := decodeManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	original := manifest.Models["minicpm-v-4.7"]
	for _, mutation := range []string{"missing", "revision", "hash", "target", "target-case", "repo", "backend"} {
		t.Run(mutation, func(t *testing.T) {
			m := original
			m.Projectors = map[string]Artifact{}
			for k, v := range original.Projectors {
				m.Projectors[k] = v
			}
			a := m.Projectors[m.Default]
			switch mutation {
			case "missing":
				delete(m.Projectors, m.Default)
			case "revision":
				a.Revision = strings.Repeat("a", 40)
			case "hash":
				a.SHA256 = ""
			case "target":
				a.Target = "../escape.gguf"
			case "target-case":
				a.Target = strings.ToLower(m.Files[m.Default])
			case "repo":
				a.Repo = "owner/other"
			case "backend":
				m.Backends = map[string]string{m.Default: ""}
			}
			if mutation != "missing" {
				m.Projectors[m.Default] = a
			}
			if err := validateModel(m.Name, m); err == nil {
				t.Fatal("accepted invalid projector metadata")
			}
		})
	}
}
