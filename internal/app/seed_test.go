package app

import (
	"bytes"
	"errors"
	"testing"
)

func TestResolveMediaSeed(t *testing.T) {
	seed, err := resolveMediaSeedFrom(0, false, false, nil)
	if err != nil || seed != 0 {
		t.Fatalf("default seed = %d, %v; want 0, nil", seed, err)
	}
	seed, err = resolveMediaSeedFrom(42, true, false, nil)
	if err != nil || seed != 42 {
		t.Fatalf("explicit seed = %d, %v; want 42, nil", seed, err)
	}

	source := bytes.NewReader([]byte{0, 0, 0, 0, 0, 0, 0x12, 0x34})
	seed, err = resolveMediaSeedFrom(0, false, true, source)
	if err != nil || seed != 0x1234 {
		t.Fatalf("random seed = %d, %v; want %d, nil", seed, err, 0x1234)
	}
}

func TestResolveMediaSeedRejectsConflictingFlags(t *testing.T) {
	if _, err := resolveMediaSeedFrom(42, true, true, nil); err == nil {
		t.Fatal("expected --seed with --random-seed to fail")
	}
}

func TestResolveMediaSeedReportsRandomSourceFailure(t *testing.T) {
	_, err := resolveMediaSeedFrom(0, false, true, errorReader{})
	if err == nil {
		t.Fatal("expected random source failure")
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("unavailable")
}
