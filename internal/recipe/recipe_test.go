package recipe

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoad(t *testing.T) {
	home := t.TempDir()
	want := Recipe{
		Name: "cinematic-video", Base: "wan2.2-video:5b-q8-mlx",
		Adapters: []string{"hf://creator/motion@0.8"}, Preset: "balanced",
	}
	if err := Save(home, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(home, want.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Base != want.Base || got.Preset != want.Preset || len(got.Adapters) != 1 {
		t.Fatalf("Load() = %#v", got)
	}
	if !Exists(home, want.Name) {
		t.Fatalf("recipe was not saved under %s", filepath.Join(home, "recipes"))
	}
}
