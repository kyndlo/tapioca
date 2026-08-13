package app

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

func resolveMediaSeed(seed uint64, seedSet, randomize bool, output io.Writer) (uint64, error) {
	return resolveMediaSeedFrom(seed, seedSet, randomize, cryptorand.Reader, output)
}

func resolveMediaSeedFrom(
	seed uint64,
	seedSet, randomize bool,
	source io.Reader,
	output io.Writer,
) (uint64, error) {
	if seedSet && randomize {
		return 0, errors.New("--seed and --random-seed cannot be used together")
	}
	if !randomize {
		return seed, nil
	}
	var value [4]byte
	if _, err := io.ReadFull(source, value[:]); err != nil {
		return 0, fmt.Errorf("generate random seed: %w", err)
	}
	resolved := uint64(binary.BigEndian.Uint32(value[:]))
	if output != nil {
		fmt.Fprintf(output, "using random seed %d\n", resolved)
	}
	return resolved, nil
}
