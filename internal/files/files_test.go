package files

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mkZip(t *testing.T, files map[string]string) []byte {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for n, c := range files {
		w, _ := zw.Create(n)
		_, _ = w.Write([]byte(c))
	}
	_ = zw.Close()
	return b.Bytes()
}

func TestTextFindsLeads(t *testing.T) {
	txt := "Layla Hassan\nlayla@hassanlabs.org +20 100 555 0199\nsee https://hassanlabs.org/team and 0x71C7656EC7ab88b098defB751B7401B5f6d8976F\n"
	a, err := Analyze(context.Background(), "cv.txt", []byte(txt))
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != "text" || len(a.Emails) != 1 || len(a.Phones) != 1 || len(a.Wallets) != 1 {
		t.Fatalf("%+v", a)
	}
	if !contains(a.Domains, "hassanlabs.org") {
		t.Fatalf("domains %v", a.Domains)
	}
	if Lookup(a.ID) == nil {
		t.Fatal("not registered")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func TestDocxMetadataAndText(t *testing.T) {
	d := mkZip(t, map[string]string{
		"word/document.xml":   `<w:document><w:p><w:r><w:t>Contact: ceo@acme-labs.org</w:t></w:r></w:p><w:p><w:r><w:t>Hassan Labs Inc</w:t></w:r></w:p></w:document>`,
		"docProps/core.xml":   `<cp:coreProperties><dc:creator>L. Hassan</dc:creator><cp:lastModifiedBy>S. Ali</cp:lastModifiedBy></cp:coreProperties>`,
		"docProps/app.xml":    `<Properties><Application>Microsoft Office Word</Application><Company>Hassan Labs</Company></Properties>`,
		"[Content_Types].xml": `<Types/>`,
	})
	a, err := Analyze(context.Background(), "report.docx", d)
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != "docx" || a.Meta["author"] != "L. Hassan" || a.Meta["last saved by"] != "S. Ali" || a.Meta["organisation"] != "Hassan Labs" {
		t.Fatalf("meta %+v", a.Meta)
	}
	if !contains(a.Emails, "ceo@acme-labs.org") || !strings.Contains(a.Text, "Hassan Labs Inc") {
		t.Fatalf("text/emails %+v", a)
	}
}

func TestRejectsUnknownAndOversize(t *testing.T) {
	if _, err := Analyze(context.Background(), "x.bin", []byte{0, 1, 2, 3, 0, 0, 9, 9}); err == nil {
		t.Fatal("binary blob accepted")
	}
	if _, err := Analyze(context.Background(), "x", nil); err == nil {
		t.Fatal("empty file accepted")
	}
}

func TestImageUpload(t *testing.T) {
	im := image.NewRGBA(image.Rect(0, 0, 120, 80))
	for x := 0; x < 120; x++ {
		for y := 0; y < 80; y++ {
			im.Set(x, y, color.RGBA{uint8(x * 2), uint8(y * 3), 90, 255})
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	a, err := Analyze(context.Background(), "shot.png", b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != "image" || a.Width != 120 || !strings.HasPrefix(a.ImageURL, "upload://") || a.DHash == "" {
		t.Fatalf("%+v", a)
	}
}

func TestVideoWithFakeFFmpeg(t *testing.T) {
	dir := t.TempDir()
	probe := "#!/bin/sh\ncat <<'EOF'\n{\"format\":{\"duration\":\"60.0\",\"format_long_name\":\"QuickTime\",\"tags\":{\"creation_time\":\"2026-05-01T10:00:00Z\",\"location\":\"+30.0444+031.2357/\",\"com.apple.quicktime.model\":\"iPhone 14\"}},\"streams\":[{\"codec_type\":\"video\",\"codec_name\":\"h264\",\"width\":1920,\"height\":1080}]}\nEOF\n"
	// the fake ffmpeg writes two real JPEGs to the pattern given as its last argument
	jpg := func() []byte {
		im := image.NewRGBA(image.Rect(0, 0, 90, 60))
		for x := 0; x < 90; x++ {
			im.Set(x, x%60, color.White)
		}
		var b bytes.Buffer
		_ = jpeg.Encode(&b, im, nil)
		return b.Bytes()
	}()
	_ = os.WriteFile(filepath.Join(dir, "frame.jpg"), jpg, 0o600)
	ff := "#!/bin/sh\nfor a; do last=$a; done\nd=$(dirname \"$last\")\ncp " + filepath.Join(dir, "frame.jpg") + " \"$d/f001.jpg\"\ncp " + filepath.Join(dir, "frame.jpg") + " \"$d/f002.jpg\"\n"
	_ = os.WriteFile(filepath.Join(dir, "ffprobe"), []byte(probe), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "ffmpeg"), []byte(ff), 0o755)
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	mp4 := append([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, make([]byte, 300)...)
	a, err := Analyze(context.Background(), "clip.mp4", mp4)
	if err != nil {
		t.Fatal(err)
	}
	if a.Kind != "video" || len(a.Frames) != 2 || a.Duration != 60 || a.GPS != "30.04440,31.23570" || a.Meta["creation_time"] == "" {
		t.Fatalf("%+v", a)
	}
}

func TestVideoWithoutFFmpegSaysSo(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	mp4 := append([]byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'i', 's', 'o', 'm'}, make([]byte, 300)...)
	a, err := Analyze(context.Background(), "clip.mp4", mp4)
	if err != nil || len(a.Notes) == 0 || !strings.Contains(a.Notes[0], "ffmpeg") {
		t.Fatalf("%v %+v", err, a)
	}
}
