package protocol_test

import (
	"encoding/base64"
	"testing"

	"clipshare/src/internal/clip"
	"clipshare/src/internal/consts"
	"clipshare/src/internal/protocol"
)

func TestContentMsgTextRoundTrip(t *testing.T) {
	original := clip.Content{Kind: clip.KindText, Text: "hello world"}
	msg := protocol.ContentMsg(original, 1234, "desktop")

	if msg.Type != protocol.ContentText {
		t.Errorf("type: got %q, want %q", msg.Type, protocol.ContentText)
	}
	if msg.Text != original.Text {
		t.Errorf("text: got %q, want %q", msg.Text, original.Text)
	}
	if msg.From != "desktop" {
		t.Errorf("from: got %q, want desktop", msg.From)
	}
	if msg.Ts != 1234 {
		t.Errorf("ts: got %d, want 1234", msg.Ts)
	}

	back, ok := protocol.MsgToContent(msg)
	if !ok {
		t.Fatal("MsgToContent returned false")
	}
	if back.Kind != clip.KindText || back.Text != original.Text {
		t.Errorf("round-trip: got %+v", back)
	}
}

func TestContentMsgImageRoundTrip(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4e, 0x47}
	original := clip.Content{Kind: clip.KindImage, Image: data, Mime: "image/png"}
	msg := protocol.ContentMsg(original, 5678, "phone")

	if msg.Type != protocol.ContentImage {
		t.Errorf("type: got %q, want %q", msg.Type, protocol.ContentImage)
	}
	if msg.Mime != "image/png" {
		t.Errorf("mime: got %q, want image/png", msg.Mime)
	}
	wantData := base64.StdEncoding.EncodeToString(data)
	if msg.Data != wantData {
		t.Errorf("data: got %q, want %q", msg.Data, wantData)
	}

	back, ok := protocol.MsgToContent(msg)
	if !ok {
		t.Fatal("MsgToContent returned false")
	}
	if back.Kind != clip.KindImage {
		t.Errorf("kind: got %v, want image", back.Kind)
	}
	if string(back.Image) != string(data) {
		t.Errorf("image bytes mismatch")
	}
	if back.Mime != "image/png" {
		t.Errorf("mime: got %q, want image/png", back.Mime)
	}
}

func TestMsgToContentDefaultsMime(t *testing.T) {
	msg := protocol.ClipboardMsg{
		Type: protocol.ContentImage,
		Data: base64.StdEncoding.EncodeToString([]byte{0x01, 0x02}),
		From: "x",
	}
	back, ok := protocol.MsgToContent(msg)
	if !ok {
		t.Fatal("MsgToContent returned false")
	}
	if back.Mime != consts.DefaultImageMime {
		t.Errorf("mime: got %q, want %q", back.Mime, consts.DefaultImageMime)
	}
}

func TestMsgToContentEmptyText(t *testing.T) {
	msg := protocol.ClipboardMsg{Type: protocol.ContentText, Text: "", From: "x"}
	if _, ok := protocol.MsgToContent(msg); ok {
		t.Error("expected empty text to be rejected")
	}
}

func TestMsgToContentBadBase64(t *testing.T) {
	msg := protocol.ClipboardMsg{Type: protocol.ContentImage, Data: "!!!not-base64!!!", From: "x"}
	if _, ok := protocol.MsgToContent(msg); ok {
		t.Error("expected bad base64 to be rejected")
	}
}

func TestContentKeyDeterministic(t *testing.T) {
	a := protocol.ContentKey(clip.Content{Kind: clip.KindText, Text: "same"})
	b := protocol.ContentKey(clip.Content{Kind: clip.KindText, Text: "same"})
	if a != b {
		t.Errorf("same content produced different keys: %d vs %d", a, b)
	}

	c := protocol.ContentKey(clip.Content{Kind: clip.KindText, Text: "different"})
	if a == c {
		t.Error("different text produced same key")
	}

	img := protocol.ContentKey(clip.Content{Kind: clip.KindImage, Image: []byte{0x01}, Mime: "image/png"})
	if a == img {
		t.Error("text and image produced same key")
	}
}
