package app

import "github.com/carlos/tapioca/internal/catalog"

func verifyArtifact(path string, expected catalog.Download) error {
	return catalog.VerifyArtifact(path, expected)
}
