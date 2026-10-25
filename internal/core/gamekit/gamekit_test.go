package gamekit

import (
	"strings"
	"testing"
)

const page = "<!DOCTYPE html><html><head><title>t</title></head><body><script>GameKit.setup()</script></body></html>"

func TestEmbedAfterHeadAndStripRoundTrip(t *testing.T) {
	out := Embed(page)
	if !strings.Contains(out, "<head>"+openTag) {
		t.Fatal("GameKit must be right after <head>")
	}
	if strings.Count(Embed(out), openTag) != 1 {
		t.Fatal("re-embedding must not duplicate GameKit")
	}
	if Strip(out) != page {
		t.Fatal("strip must restore the original")
	}
}

func TestEmbedWithoutHead(t *testing.T) {
	out := Embed("<html><body>x</body></html>")
	if !strings.Contains(out, "<head>"+openTag) || !strings.Contains(out, "<body>x") {
		t.Fatalf("unexpected: %.120s", out)
	}
}

func TestSourceHasNoClosingScriptTag(t *testing.T) {
	if strings.Contains(strings.ToLower(js), "</script") {
		t.Fatal("gamekit.js must not contain </script>, it would break inline embedding")
	}
}
