package hotkey

import (
	"testing"
	"time"
)

func TestDetector_RecordAndHeat(t *testing.T) {
	d := NewDetector(DefaultConfig())
	key := "video:entity:42"

	if d.Heat(key) != 0 {
		t.Fatal("expected 0 heat for unseen key")
	}

	d.Record(key)
	if d.Heat(key) != 1 {
		t.Fatal("expected 1 after one record")
	}

	for i := 0; i < 49; i++ {
		d.Record(key)
	}
	if d.Heat(key) != 50 {
		t.Fatalf("expected 50, got %d", d.Heat(key))
	}
}

func TestDetector_Level(t *testing.T) {
	d := NewDetector(DefaultConfig())
	key := "test:key"

	if d.Level(key) != LevelNone {
		t.Fatal("expected NONE for unseen key")
	}

	for i := 0; i < 50; i++ {
		d.Record(key)
	}
	if d.Level(key) != LevelLow {
		t.Fatal("expected LOW at 50")
	}

	for i := 0; i < 150; i++ {
		d.Record(key)
	}
	if d.Level(key) != LevelMedium {
		t.Fatal("expected MEDIUM at 200")
	}

	for i := 0; i < 300; i++ {
		d.Record(key)
	}
	if d.Level(key) != LevelHigh {
		t.Fatal("expected HIGH at 500")
	}
}

func TestDetector_ExtendTTL(t *testing.T) {
	d := NewDetector(DefaultConfig())
	base := 60 * time.Second
	key := "test:key"

	if d.ExtendTTL(base, key) != base {
		t.Fatal("expected base TTL for NONE")
	}

	for i := 0; i < 50; i++ {
		d.Record(key)
	}
	if d.ExtendTTL(base, key) != base+20*time.Second {
		t.Fatal("expected +20s for LOW")
	}
}

func TestDetector_Rotate(t *testing.T) {
	cfg := DefaultConfig()
	cfg.WindowSeconds = 20
	cfg.SegmentSeconds = 10
	d := NewDetector(cfg) // 2 segments
	key := "test:key"

	// Record in segment 0
	for i := 0; i < 10; i++ {
		d.Record(key)
	}
	if d.Heat(key) != 10 {
		t.Fatal("expected 10 before rotation")
	}

	// First rotation: advance to segment 1, clear segment 1 (already 0)
	// segment[0]=10, segment[1]=0 => heat=10
	d.rotate()
	if d.Heat(key) != 10 {
		t.Fatalf("expected 10 after one rotation, got %d", d.Heat(key))
	}

	// Second rotation: advance back to segment 0, clear segment 0
	// segment[0]=0, segment[1]=0 => heat=0
	d.rotate()
	if d.Heat(key) != 0 {
		t.Fatalf("expected 0 after full window rotation, got %d", d.Heat(key))
	}
}

func TestDetector_Reset(t *testing.T) {
	d := NewDetector(DefaultConfig())
	key := "test:key"
	d.Record(key)
	d.Reset(key)
	if d.Heat(key) != 0 {
		t.Fatal("expected 0 after reset")
	}
}
