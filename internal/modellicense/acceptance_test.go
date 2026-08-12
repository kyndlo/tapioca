package modellicense

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcceptAndRequire(t *testing.T) {
	home := t.TempDir()
	t.Setenv("TAPIOCA_HOME", home)
	if err := Require("krea-2-turbo:bf16-cuda", "Krea 2 Community License", "https://example.test"); err == nil {
		t.Fatal("Require() succeeded before acceptance")
	}
	if err := Accept("krea-2-turbo:bf16-cuda", "Krea 2 Community License", "https://example.test"); err != nil {
		t.Fatal(err)
	}
	if err := Require("krea-2-turbo:bf16-cuda", "Krea 2 Community License", "https://example.test"); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(home, "licenses.json"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("licenses.json permissions = %o", info.Mode().Perm())
	}
}
