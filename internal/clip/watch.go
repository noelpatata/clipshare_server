package clip

import (
	"context"
	"hash/fnv"
	"log"
	"time"
)

// Watcher polls the clipboard for external changes and fires onChange with
// the new text. It applies loop protection: content written by the local
// client (via Track/LocalWrite) is not reported again.
type Watcher struct {
	clip      Interface
	interval  time.Duration
	onChange  func(string)
	lastHash  uint64
	skipHash  uint64
}

func NewWatcher(c Interface, interval time.Duration, onChange func(string)) *Watcher {
	return &Watcher{clip: c, interval: interval, onChange: onChange}
}

func hash(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// Run polls the clipboard until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	w.lastHash = w.snapshot()
	t := time.NewTicker(w.interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			text, err := w.clip.Read()
			if err != nil {
				log.Printf("clipboard read: %v", err)
				continue
			}
			h := hash(text)
			if h == w.lastHash {
				continue
			}
			w.lastHash = h
			if h == w.skipHash {
				continue
			}
			w.onChange(text)
		}
	}
}

// LocalWrite writes text to the clipboard and marks it so the watcher does
// not rebroadcast it as an external change.
func (w *Watcher) LocalWrite(text string) {
	if err := w.clip.Write(text); err != nil {
		log.Printf("clipboard write: %v", err)
		return
	}
	w.skipHash = hash(text)
	w.lastHash = hash(text)
}

// Track records a remote value as the current local state without writing.
func (w *Watcher) Track(text string) {
	w.skipHash = hash(text)
	w.lastHash = hash(text)
}

func (w *Watcher) snapshot() uint64 {
	text, err := w.clip.Read()
	if err != nil {
		return 0
	}
	return hash(text)
}
