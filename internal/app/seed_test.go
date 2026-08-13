package app

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestResolveMediaSeed(t *testing.T) {
	seed, err := resolveMediaSeedFrom(0, false, false, nil, nil)
	if err != nil || seed != 0 {
		t.Fatalf("default seed = %d, %v; want 0, nil", seed, err)
	}
	seed, err = resolveMediaSeedFrom(42, true, false, nil, nil)
	if err != nil || seed != 42 {
		t.Fatalf("explicit seed = %d, %v; want 42, nil", seed, err)
	}

	source := bytes.NewReader([]byte{0, 0, 0x12, 0x34})
	var output bytes.Buffer
	seed, err = resolveMediaSeedFrom(0, false, true, source, &output)
	if err != nil || seed != 0x1234 {
		t.Fatalf("random seed = %d, %v; want %d, nil", seed, err, 0x1234)
	}
	if got := strings.TrimSpace(output.String()); got != "using random seed 4660" {
		t.Fatalf("printed seed = %q, want returned seed %d", got, seed)
	}

	seed, err = resolveMediaSeedFrom(
		0, false, true, bytes.NewReader([]byte{0xff, 0xff, 0xff, 0xff}), nil,
	)
	if err != nil || seed != math.MaxUint32 {
		t.Fatalf("maximum random seed = %d, %v; want %d, nil", seed, err, uint64(math.MaxUint32))
	}
}

func TestResolveMediaSeedRejectsConflictingFlags(t *testing.T) {
	if _, err := resolveMediaSeedFrom(42, true, true, nil, nil); err == nil {
		t.Fatal("expected --seed with --random-seed to fail")
	}
}

func TestResolveMediaSeedReportsRandomSourceFailure(t *testing.T) {
	_, err := resolveMediaSeedFrom(0, false, true, errorReader{}, nil)
	if err == nil {
		t.Fatal("expected random source failure")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("unavailable")
}
