package hotkey

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Level int

const (
	LevelNone   Level = iota
	LevelLow        // >= 50 accesses in window
	LevelMedium     // >= 200
	LevelHigh       // >= 500
)

type Config struct {
	WindowSeconds  int           // sliding window size (default 60)
	SegmentSeconds int           // bucket rotation interval (default 10)
	ThresholdLow   int           // accesses to reach LOW (default 50)
	ThresholdMed   int           // accesses to reach MEDIUM (default 200)
	ThresholdHigh  int           // accesses to reach HIGH (default 500)
	ExtendLow      time.Duration // TTL extension for LOW (default 20s)
	ExtendMed      time.Duration // TTL extension for MEDIUM (default 60s)
	ExtendHigh     time.Duration // TTL extension for HIGH (default 120s)
}

func DefaultConfig() Config {
	return Config{
		WindowSeconds:  60,
		SegmentSeconds: 10,
		ThresholdLow:   50,
		ThresholdMed:   200,
		ThresholdHigh:  500,
		ExtendLow:      20 * time.Second,
		ExtendMed:      60 * time.Second,
		ExtendHigh:     120 * time.Second,
	}
}

type bucket struct {
	counts []int
}

type Detector struct {
	counters map[string]*bucket
	mu       sync.RWMutex
	current  atomic.Int32
	segments int
	cfg      Config
}

func NewDetector(cfg Config) *Detector {
	if cfg.WindowSeconds <= 0 {
		cfg.WindowSeconds = 60
	}
	if cfg.SegmentSeconds <= 0 {
		cfg.SegmentSeconds = 10
	}
	segments := cfg.WindowSeconds / cfg.SegmentSeconds
	if segments < 1 {
		segments = 1
	}
	return &Detector{
		counters: make(map[string]*bucket),
		segments: segments,
		cfg:      cfg,
	}
}

func (d *Detector) Record(key string) {
	idx := int(d.current.Load())
	b := d.getOrCreateBucket(key)
	b.counts[idx]++
}

func (d *Detector) Heat(key string) int {
	d.mu.RLock()
	b, ok := d.counters[key]
	d.mu.RUnlock()
	if !ok {
		return 0
	}
	sum := 0
	for _, v := range b.counts {
		sum += v
	}
	return sum
}

func (d *Detector) Level(key string) Level {
	h := d.Heat(key)
	switch {
	case h >= d.cfg.ThresholdHigh:
		return LevelHigh
	case h >= d.cfg.ThresholdMed:
		return LevelMedium
	case h >= d.cfg.ThresholdLow:
		return LevelLow
	default:
		return LevelNone
	}
}

func (d *Detector) ExtendTTL(base time.Duration, key string) time.Duration {
	return base + d.extendSeconds(d.Level(key))
}

func (d *Detector) StartRotation(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(d.cfg.SegmentSeconds) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.rotate()
		}
	}
}

func (d *Detector) rotate() {
	next := (int(d.current.Load()) + 1) % d.segments
	d.current.Store(int32(next))
	d.mu.Lock()
	for _, b := range d.counters {
		b.counts[next] = 0
	}
	d.mu.Unlock()
}

func (d *Detector) Reset(key string) {
	d.mu.Lock()
	delete(d.counters, key)
	d.mu.Unlock()
}

func (d *Detector) getOrCreateBucket(key string) *bucket {
	d.mu.RLock()
	b, ok := d.counters[key]
	d.mu.RUnlock()
	if ok {
		return b
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	b, ok = d.counters[key]
	if !ok {
		b = &bucket{counts: make([]int, d.segments)}
		d.counters[key] = b
	}
	return b
}

func (d *Detector) extendSeconds(l Level) time.Duration {
	switch l {
	case LevelHigh:
		return d.cfg.ExtendHigh
	case LevelMedium:
		return d.cfg.ExtendMed
	case LevelLow:
		return d.cfg.ExtendLow
	default:
		return 0
	}
}
