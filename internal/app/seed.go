package app

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func resolveMediaSeed(seed uint64, seedSet, randomize bool) (uint64, error) {
	return resolveMediaSeedFrom(seed, seedSet, randomize, cryptorand.Reader)
}

func resolveMediaSeedFrom(seed uint64, seedSet, randomize bool, source io.Reader) (uint64, error) {
	if seedSet && randomize {
		return 0, errors.New("--seed and --random-seed cannot be used together")
	}
	if !randomize {
		return seed, nil
	}
	var value [8]byte
	if _, err := io.ReadFull(source, value[:]); err != nil {
		return 0, fmt.Errorf("generate random seed: %w", err)
	}
	return binary.BigEndian.Uint64(value[:]), nil
}
