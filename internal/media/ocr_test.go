// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package media

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestCleanOCRTesseract(t *testing.T) {
	got := cleanOCR("  John   Smith \n\n ~~ \nACME Corp\n.\n", "tesseract")
	if got != "John Smith\nACME Corp" {
		t.Fatalf("got %q", got)
	}
}

func TestCleanOCREasyDetail(t *testing.T) {
	in := "([[0, 0], [10, 0], [10, 5], [0, 5]], 'Layla Hassan', 0.98)\n([[0, 9]], 'x', 0.2)\n"
	if got := cleanOCR(in, "easyocr"); got != "Layla Hassan" {
		t.Fatalf("got %q", got)
	}
}

func TestCleanOCRCaps(t *testing.T) {
	got := cleanOCR(strings.Repeat("word ", 400), "tesseract")
	if r := []rune(got); len(r) > maxOCRText+1 {
		t.Fatalf("not capped: %d", len(r))
	}
}

func TestReadOCRWithFakeTesseract(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\ncat >/dev/null\nprintf 'Layla Hassan\\nHassan Labs Inc\\n'\n"
	if err := os.WriteFile(dir+"/tesseract", []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	ocrOnce = sync.Once{}
	ocrBin, ocrAlt = "", ""
	img := &Image{Width: 400, Height: 300, Data: make([]byte, 1000), ContentType: "image/png"}
	readOCR(context.Background(), img)
	if got := img.Meta["OCRText"]; got != "Layla Hassan\nHassan Labs Inc" {
		t.Fatalf("got %q", got)
	}
	ocrOnce = sync.Once{}
	ocrBin, ocrAlt = "", ""
}
