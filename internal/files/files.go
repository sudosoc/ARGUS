// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package files analyses what the operator uploads as an input: pictures,
// documents and videos. Everything is read locally; nothing is sent to any
// service. The result is kept in a process-wide registry so a scan can refer to
// an upload by id (Target.Files) and the files module can turn it into findings.
package files

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sudosoc/argus/internal/cvparse"
	"github.com/sudosoc/argus/internal/doc"
	"github.com/sudosoc/argus/internal/extract"
	"github.com/sudosoc/argus/internal/media"
	"github.com/sudosoc/argus/internal/modules/docmeta"
)

// Size limits (bytes) by kind.
const (
	MaxImage    = 100 << 20
	MaxDocument = 100 << 20
	MaxVideo    = 500 << 20
	maxText     = 200_000
)

// Frame is one picture taken out of a video.
type Frame struct {
	AtSeconds float64           `json:"at_seconds"`
	URL       string            `json:"url"` // upload://… so the image proxy can show it
	OCR       string            `json:"ocr,omitempty"`
	SHA256    string            `json:"sha256"`
	DHash     string            `json:"dhash,omitempty"`
	GPS       string            `json:"gps,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// Analysis is everything learned from one uploaded file.
type Analysis struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Kind      string            `json:"kind"` // image, pdf, docx, xlsx, pptx, odf, video, text
	Mime      string            `json:"mime"`
	Size      int               `json:"size"`
	SHA256    string            `json:"sha256"`
	Uploaded  time.Time         `json:"uploaded"`
	Meta      map[string]string `json:"meta,omitempty"`
	Text      string            `json:"text,omitempty"`
	TextChars int               `json:"text_chars,omitempty"`
	Emails    []string          `json:"emails,omitempty"`
	Phones    []string          `json:"phones,omitempty"`
	Domains   []string          `json:"domains,omitempty"`
	URLs      []string          `json:"urls,omitempty"`
	Wallets   []string          `json:"wallets,omitempty"`
	Name_     string            `json:"person_name,omitempty"` // a name the text suggests (CVs, bios)
	Employers []string          `json:"employers,omitempty"`
	Location  string            `json:"location,omitempty"`
	// Image
	ImageURL string  `json:"image_url,omitempty"`
	Width    int     `json:"width,omitempty"`
	Height   int     `json:"height,omitempty"`
	GPS      string  `json:"gps,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
	Taken    string  `json:"taken,omitempty"`
	Camera   string  `json:"camera,omitempty"`
	DHash    string  `json:"dhash,omitempty"`
	// Video
	Frames   []Frame  `json:"frames,omitempty"`
	Duration float64  `json:"duration_seconds,omitempty"`
	Notes    []string `json:"notes,omitempty"` // what could not be done and why (no ffmpeg, scanned PDF…)
}

// ---------------------------------------------------------------- registry

var (
	regMu sync.RWMutex
	reg   = map[string]*Analysis{}
)

// Lookup returns the analysis of an upload by id.
func Lookup(id string) *Analysis {
	regMu.RLock()
	defer regMu.RUnlock()
	return reg[id]
}

func register(a *Analysis) {
	regMu.Lock()
	defer regMu.Unlock()
	if len(reg) > 500 { // a long-running server: forget the oldest
		var oldest string
		var ot time.Time
		for id, x := range reg {
			if oldest == "" || x.Uploaded.Before(ot) {
				oldest, ot = id, x.Uploaded
			}
		}
		delete(reg, oldest)
	}
	reg[a.ID] = a
}

// ------------------------------------------------------------------ analyse

// Kind decides what a file is from its content (never from the name alone) and returns its size limit.
func Kind(name string, data []byte) (kind, mime string, limit int) {
	head := data[:min(len(data), 4096)]
	mime = http.DetectContentType(head)
	switch {
	case strings.HasPrefix(mime, "image/"):
		return "image", mime, MaxImage
	case bytes.HasPrefix(head, []byte("%PDF")):
		return "pdf", "application/pdf", MaxDocument
	case bytes.HasPrefix(head, []byte("PK\x03\x04")):
		k := zipKind(data)
		return k, "application/zip", MaxDocument
	case len(head) > 12 && (string(head[4:8]) == "ftyp" || bytes.HasPrefix(head, []byte{0x1A, 0x45, 0xDF, 0xA3}) || (bytes.HasPrefix(head, []byte("RIFF")) && string(head[8:12]) == "AVI ")):
		return "video", "video/*", MaxVideo
	case isText(head):
		return "text", "text/plain", MaxDocument
	}
	return "", mime, 0
}

func isText(b []byte) bool {
	if len(b) == 0 || bytes.IndexByte(b, 0) >= 0 {
		return false
	}
	bad := 0
	for _, c := range b {
		if c < 9 || (c > 13 && c < 32) {
			bad++
		}
	}
	return bad*50 < len(b)
}

func zipKind(data []byte) string {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	for _, f := range zr.File {
		switch {
		case strings.HasPrefix(f.Name, "word/"):
			return "docx"
		case strings.HasPrefix(f.Name, "xl/"):
			return "xlsx"
		case strings.HasPrefix(f.Name, "ppt/"):
			return "pptx"
		case f.Name == "mimetype":
			return "odf"
		}
	}
	return ""
}

// Analyze reads one file. ctx bounds external helpers (OCR, ffmpeg).
func Analyze(ctx context.Context, name string, data []byte) (*Analysis, error) {
	if len(data) == 0 {
		return nil, errors.New("the file is empty")
	}
	kind, mime, limit := Kind(name, data)
	if kind == "" {
		return nil, errors.New("unsupported file type (pictures, PDF, Word, Excel, PowerPoint, OpenDocument, text and video are accepted)")
	}
	if len(data) > limit {
		return nil, fmt.Errorf("the file is larger than the %d MB limit for this type", limit>>20)
	}
	sum := sha256.Sum256(data)
	id := make([]byte, 8)
	_, _ = rand.Read(id)
	a := &Analysis{ID: hex.EncodeToString(id), Name: cleanName(name), Kind: kind, Mime: mime, Size: len(data), SHA256: hex.EncodeToString(sum[:]), Uploaded: time.Now().UTC()}
	switch kind {
	case "image":
		if err := analyseImage(ctx, a, data); err != nil {
			return nil, err
		}
	case "pdf":
		a.Meta = docmeta.FileMeta(".pdf", data)
		t, err := doc.PDFText(data)
		if err != nil {
			a.Notes = append(a.Notes, "no text could be read from the PDF: "+err.Error())
		}
		a.Text = t
	case "docx", "xlsx", "pptx", "odf":
		ext := map[string]string{"docx": ".docx", "xlsx": ".xlsx", "pptx": ".pptx", "odf": ".odt"}[kind]
		a.Meta = docmeta.FileMeta(ext, data)
		a.Text = zipText(kind, data)
		if a.Text == "" {
			a.Notes = append(a.Notes, "no text found in the document")
		}
	case "text":
		a.Text = string(data[:min(len(data), maxText)])
	case "video":
		analyseVideo(ctx, a, data)
	}
	finishText(a)
	register(a)
	return a, nil
}

func cleanName(n string) string {
	n = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r < 32 {
			return '_'
		}
		return r
	}, n)
	if len(n) > 120 {
		n = n[len(n)-120:]
	}
	if n == "" {
		n = "file"
	}
	return n
}

func analyseImage(ctx context.Context, a *Analysis, data []byte) error {
	img, err := media.AnalyseBytes(ctx, a.Name, data, MaxImage)
	if err != nil {
		return err
	}
	a.ImageURL, a.Width, a.Height, a.Taken, a.Camera, a.DHash = img.URL, img.Width, img.Height, img.Taken, img.Camera, img.DHashHex()
	if img.HasGPS {
		a.Lat, a.Lon = img.Lat, img.Lon
		a.GPS = fmt.Sprintf("%.5f,%.5f", img.Lat, img.Lon)
	}
	a.Meta = map[string]string{}
	for k, v := range img.Meta {
		if k == "OCRText" {
			a.Text = v
			continue
		}
		a.Meta[k] = v
	}
	if a.Text == "" {
		if e, _ := media.OCRPath(); e == "" {
			a.Notes = append(a.Notes, "OCR is off: install tesseract (or easyocr) to read text printed in pictures")
		}
	}
	return nil
}

var (
	tagStrip  = regexp.MustCompile(`<[^>]+>`)
	paraBreak = regexp.MustCompile(`</(w:p|text:p|text:h|a:p|row|si)>`)
	urlRe     = regexp.MustCompile(`https?://[^\s"'<>)\]]{4,200}`)
	walletRe  = regexp.MustCompile(`\b(0x[0-9a-fA-F]{40}|bc1[0-9a-z]{25,60}|[13][1-9A-HJ-NP-Za-km-z]{25,34})\b`)
)

// zipText reads the text parts of an Office or OpenDocument file.
func zipText(kind string, data []byte) string {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return ""
	}
	var parts []*zip.File
	for _, f := range zr.File {
		n := f.Name
		switch kind {
		case "docx":
			if n == "word/document.xml" || strings.HasPrefix(n, "word/header") || strings.HasPrefix(n, "word/footer") || n == "word/footnotes.xml" {
				parts = append(parts, f)
			}
		case "xlsx":
			if n == "xl/sharedStrings.xml" {
				parts = append(parts, f)
			}
		case "pptx":
			if strings.HasPrefix(n, "ppt/slides/slide") && strings.HasSuffix(n, ".xml") || strings.HasPrefix(n, "ppt/notesSlides/") {
				parts = append(parts, f)
			}
		case "odf":
			if n == "content.xml" {
				parts = append(parts, f)
			}
		}
	}
	sort.Slice(parts, func(i, j int) bool { return parts[i].Name < parts[j].Name })
	var sb strings.Builder
	for _, f := range parts {
		rc, err := f.Open()
		if err != nil {
			continue
		}
		b, _ := io.ReadAll(io.LimitReader(rc, 8<<20))
		rc.Close()
		s := paraBreak.ReplaceAllString(string(b), "\n")
		s = tagStrip.ReplaceAllString(s, " ")
		sb.WriteString(html.UnescapeString(s))
		sb.WriteByte('\n')
		if sb.Len() > maxText {
			break
		}
	}
	return squeeze(sb.String())
}

func squeeze(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		l = strings.Join(strings.Fields(l), " ")
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// finishText finds the leads in the text and in the metadata.
func finishText(a *Analysis) {
	text := a.Text
	if len(text) > maxText {
		text = text[:maxText]
	}
	a.TextChars = len(a.Text)
	meta := ""
	for _, v := range a.Meta {
		meta += " " + v
	}
	all := text + "\n" + meta
	a.Emails = extract.Emails(all)
	a.Phones = extract.Phones(all)
	seenDom := map[string]bool{}
	for _, u := range urlRe.FindAllString(all, -1) {
		u = strings.TrimRight(u, ".,;:")
		a.URLs = appendUniq(a.URLs, u)
		if pu, err := url.Parse(u); err == nil && pu.Hostname() != "" && !seenDom[pu.Hostname()] {
			seenDom[pu.Hostname()] = true
			a.Domains = append(a.Domains, strings.ToLower(pu.Hostname()))
		}
	}
	for _, e := range a.Emails {
		if _, d, ok := strings.Cut(e, "@"); ok && !seenDom[d] {
			seenDom[d] = true
			a.Domains = append(a.Domains, d)
		}
	}
	sort.Strings(a.Domains)
	for _, w := range walletRe.FindAllString(all, -1) {
		a.Wallets = appendUniq(a.Wallets, w)
	}
	if a.Kind == "docx" || a.Kind == "pdf" || a.Kind == "text" {
		cv := cvparse.Parse(text)
		a.Name_, a.Employers, a.Location = cv.Name, cv.Employers, cv.Location
	}
	if len(a.Text) > 20000 {
		a.Text = a.Text[:20000] + "…"
	}
}

func appendUniq(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}
