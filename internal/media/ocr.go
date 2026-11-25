// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package media

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ocrOnce sync.Once
	ocrBin  string // tesseract
	ocrAlt  string // easyocr CLI, used only when tesseract is missing
	ocrSem  = make(chan struct{}, 2)
)

// OCRPath reports which OCR engine is available: "tesseract", "easyocr" or "".
func OCRPath() (engine, path string) {
	ocrOnce.Do(func() {
		ocrBin, _ = exec.LookPath("tesseract")
		ocrAlt, _ = exec.LookPath("easyocr")
	})
	if ocrBin != "" {
		return "tesseract", ocrBin
	}
	if ocrAlt != "" {
		return "easyocr", ocrAlt
	}
	return "", ""
}

// ocrLangs is the language set handed to the engine. Tesseract writes it as
// "eng+ara"; EasyOCR wants "en ar". English and Arabic are used when Tesseract has the Arabic data;
// set ARGUS_OCR_LANGS (for example "eng+ara+fra") to choose others.
func ocrLangs() string {
	if v := strings.TrimSpace(os.Getenv("ARGUS_OCR_LANGS")); v != "" {
		return v
	}
	arOnce.Do(func() {
		if ocrBin == "" {
			return
		}
		out, err := exec.Command(ocrBin, "--list-langs").Output()
		if err == nil {
			for _, l := range strings.Fields(string(out)) {
				if l == "ara" {
					arabic = true
				}
			}
		}
	})
	if arabic {
		return "eng+ara" // English and Arabic by default when the Arabic data is installed
	}
	return "eng"
}

var (
	arOnce sync.Once
	arabic bool
)

const maxOCRText = 600

// readOCR reads text printed in a picture (signs, badges, screenshots,
// name tags) with Tesseract, or EasyOCR when only that is installed. The text
// goes into Meta["OCRText"]. It is a no-op without an engine, for tiny
// pictures and for formats the engines cannot read.
func readOCR(ctx context.Context, img *Image) {
	engine, bin := OCRPath()
	if engine == "" || img.Width < 80 || img.Height < 40 || len(img.Data) < 256 || img.ContentType == "image/svg+xml" {
		return
	}
	select {
	case ocrSem <- struct{}{}:
		defer func() { <-ocrSem }()
	case <-ctx.Done():
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	var text string
	switch engine {
	case "tesseract":
		cmd := exec.CommandContext(ctx, bin, "stdin", "stdout", "-l", ocrLangs(), "--psm", "6")
		cmd.Stdin = bytes.NewReader(img.Data)
		var out bytes.Buffer
		cmd.Stdout = &out
		_ = cmd.Run() // Tesseract exits non-zero on unreadable pictures; whatever it printed is still used
		text = out.String()
	case "easyocr":
		f, err := os.CreateTemp("", "argus-ocr-*")
		if err != nil {
			return
		}
		defer os.Remove(f.Name())
		_, werr := f.Write(img.Data)
		f.Close()
		if werr != nil {
			return
		}
		cmd := exec.CommandContext(ctx, bin, "-l", strings.Join(strings.FieldsFunc(ocrLangs(), func(r rune) bool { return r == '+' || r == ' ' || r == ',' }), " "), "-f", f.Name(), "--detail=0")
		var out bytes.Buffer
		cmd.Stdout = &out
		_ = cmd.Run()
		text = out.String()
	}
	text = cleanOCR(text, engine)
	if text == "" {
		return
	}
	if img.Meta == nil {
		img.Meta = map[string]string{}
	}
	img.Meta["OCRText"] = text
}

var easyDetail = regexp.MustCompile(`'((?:[^'\\]|\\.)*)'\s*,\s*[0-9.]+\)\s*$`)

// cleanOCR collapses whitespace, drops lines with no letters or digits and
// caps the length. EasyOCR may print "(box, 'text', 0.98)" lines; their text
// is extracted.
func cleanOCR(s, engine string) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		l = strings.TrimSpace(l)
		if engine == "easyocr" {
			if m := easyDetail.FindStringSubmatch(l); m != nil {
				l = m[1]
			}
		}
		l = strings.Join(strings.Fields(l), " ")
		if !hasAlnum(l) || len([]rune(l)) < 2 {
			continue
		}
		lines = append(lines, l)
	}
	out := strings.Join(lines, "\n")
	if r := []rune(out); len(r) > maxOCRText {
		out = string(r[:maxOCRText]) + "…"
	}
	return out
}

func hasAlnum(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r > 0x7f {
			return true
		}
	}
	return false
}
