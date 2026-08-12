package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/carlos/tapioca/internal/catalog"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "validate" {
		data, err := os.ReadFile(os.Args[2])
		if err != nil {
			fatal(err)
		}
		if err := catalog.ValidateManifest(data); err != nil {
			fatal(err)
		}
		return
	}
	root := "catalog"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	data, err := catalog.EncodeBuiltInManifest(time.Now())
	if err != nil {
		fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		fatal(err)
	}
	path := filepath.Join(root, "catalog.json")
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o644); err != nil {
		fatal(err)
	}
	digest := sha256.Sum256(data)
	checksum := hex.EncodeToString(digest[:]) + "  catalog.json\n"
	if err := os.WriteFile(path+".sha256", []byte(checksum), 0o644); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "catalog-manifest:", err)
	os.Exit(1)
}
