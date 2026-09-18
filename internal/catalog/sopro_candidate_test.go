package catalog

import (
	"os"
	"testing"
)

func TestSoproCandidateManifest(t *testing.T) {
	data, err := os.ReadFile("../../catalog/candidates/sopro-v2-turbo.json")
	if err != nil {
		t.Fatal(err)
	}
	manifest, err := decodeManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	model := manifest.Models["sopro-v2-turbo"]
	if model.Repo != "samuel-vitorino/sopro-v2-turbo" || model.Backends[model.Default] != "speech-sopro" {
		t.Fatalf("incorrect Sopro candidate: %#v", model)
	}
	artifacts := model.Artifacts[model.Default]
	if len(artifacts) != 6 {
		t.Fatalf("want six pinned Sopro artifacts, got %d", len(artifacts))
	}
	for _, artifact := range artifacts {
		if artifact.Revision != "f747f9edfb7b0233a3b7105af3a75603a7213d26" ||
			artifact.SizeBytes <= 0 || len(artifact.SHA256) != 64 {
			t.Fatalf("artifact is not pinned: %#v", artifact)
		}
	}
}
