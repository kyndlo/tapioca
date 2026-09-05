package app

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/carlos/tapioca/internal/catalog"
)

// Verify before promoting a partial download or reusing a cached artifact.
func verifyArtifact(path string, expected catalog.Download) error {
	if expected.SizeBytes == 0 && expected.SHA256 == "" {
		return nil
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if expected.SizeBytes > 0 && info.Size() != expected.SizeBytes {
		return fmt.Errorf("artifact size mismatch: got %d bytes, expected %d; retry the download with --force", info.Size(), expected.SizeBytes)
	}
	if expected.SHA256 != "" {
		digest := sha256.New()
		if _, err := io.Copy(digest, file); err != nil {
			return err
		}
		if hex.EncodeToString(digest.Sum(nil)) != expected.SHA256 {
			return fmt.Errorf("artifact SHA-256 mismatch; retry the download with --force")
		}
	}
	return nil
}
