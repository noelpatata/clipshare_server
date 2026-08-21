package clip_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"clipshare/src/internal/clip"
)

type fakeClipboard struct {
	content clip.Content
	writes  []clip.Content
	err     error
}

func (f *fakeClipboard) Read() (clip.Content, error) {
	if f.err != nil {
		return clip.Content{}, f.err
	}
	return f.content, nil
}

func (f *fakeClipboard) Write(c clip.Content) error {
	f.writes = append(f.writes, c)
	f.content = c
	return nil
}

func (f *fakeClipboard) Close() {}

func TestWatcherDetectsChange(t *testing.T) {
	fb := &fakeClipboard{content: clip.Content{Kind: clip.KindText, Text: "first"}}
	var got []string
	done := make(chan struct{})

	w := clip.NewWatcher(fb, 20*time.Millisecond, func(c clip.Content) {
		got = append(got, c.Text)
		if len(got) == 1 {
			close(done)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// Give it time to take the initial snapshot, then change the content.
	time.Sleep(60 * time.Millisecond)
	fb.content = clip.Content{Kind: clip.KindText, Text: "second"}

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timeout waiting for change detection")
	}

	cancel()

	if len(got) != 1 || got[0] != "second" {
		t.Errorf("got %v, want [second]", got)
	}
}

func TestWatcherIgnoresSameContent(t *testing.T) {
	fb := &fakeClipboard{content: clip.Content{Kind: clip.KindText, Text: "same"}}
	var got []string

	w := clip.NewWatcher(fb, 20*time.Millisecond, func(c clip.Content) {
		got = append(got, c.Text)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	// Content never changes.
	time.Sleep(100 * time.Millisecond)
	cancel()

	if len(got) != 0 {
		t.Errorf("expected no callbacks, got %v", got)
	}
}

func TestLocalWritePreventsRebroadcast(t *testing.T) {
	fb := &fakeClipboard{content: clip.Content{Kind: clip.KindText, Text: "initial"}}
	var got []string

	w := clip.NewWatcher(fb, 20*time.Millisecond, func(c clip.Content) {
		got = append(got, c.Text)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	time.Sleep(60 * time.Millisecond)
	w.LocalWrite(clip.Content{Kind: clip.KindText, Text: "remote"})

	// The fake clipboard now returns "remote", but LocalWrite should have
	// marked it as a local write, so no rebroadcast happens.
	time.Sleep(100 * time.Millisecond)
	cancel()

	if len(got) != 0 {
		t.Errorf("expected no callbacks after LocalWrite, got %v", got)
	}
	if len(fb.writes) != 1 || fb.writes[0].Text != "remote" {
		t.Errorf("writes: got %+v", fb.writes)
	}
}

func TestTrackPreventsRebroadcast(t *testing.T) {
	fb := &fakeClipboard{content: clip.Content{Kind: clip.KindText, Text: "initial"}}
	var got []string

	w := clip.NewWatcher(fb, 20*time.Millisecond, func(c clip.Content) {
		got = append(got, c.Text)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	time.Sleep(60 * time.Millisecond)
	w.Track(clip.Content{Kind: clip.KindText, Text: "remote"})
	fb.content = clip.Content{Kind: clip.KindText, Text: "remote"}

	time.Sleep(100 * time.Millisecond)
	cancel()

	if len(got) != 0 {
		t.Errorf("expected no callbacks after Track, got %v", got)
	}
}

func TestReadErrorIsLoggedAndIgnored(t *testing.T) {
	fb := &fakeClipboard{content: clip.Content{Kind: clip.KindText, Text: "ok"}, err: errors.New("boom")}
	var got []string

	w := clip.NewWatcher(fb, 20*time.Millisecond, func(c clip.Content) {
		got = append(got, c.Text)
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx)

	time.Sleep(100 * time.Millisecond)
	cancel()

	if len(got) != 0 {
		t.Errorf("expected no callbacks on read error, got %v", got)
	}
}
