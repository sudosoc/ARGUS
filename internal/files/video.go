package files

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/media"
)

const maxFrames = 12

// analyseVideo reads the container metadata with ffprobe and takes up to maxFrames pictures
// with ffmpeg, spread over the duration; each frame gets the same analysis as an uploaded picture
// (hashes, EXIF, OCR). Without ffmpeg only what can be said without it is reported.
func analyseVideo(ctx context.Context, a *Analysis, data []byte) {
	probe, _ := exec.LookPath("ffprobe")
	ffmpeg, _ := exec.LookPath("ffmpeg")
	if probe == "" && ffmpeg == "" {
		a.Notes = append(a.Notes, "ffmpeg is not installed: no frames or container metadata could be read (apt install ffmpeg)")
		return
	}
	dir, err := os.MkdirTemp("", "argus-video-")
	if err != nil {
		a.Notes = append(a.Notes, "could not create a temporary directory: "+err.Error())
		return
	}
	defer os.RemoveAll(dir)
	src := filepath.Join(dir, "input")
	if err := os.WriteFile(src, data, 0o600); err != nil {
		a.Notes = append(a.Notes, "could not write the temporary file: "+err.Error())
		return
	}
	a.Meta = map[string]string{}
	if probe != "" {
		readProbe(ctx, probe, src, a)
	} else {
		a.Notes = append(a.Notes, "ffprobe is not installed: container metadata was not read")
	}
	if ffmpeg == "" {
		a.Notes = append(a.Notes, "ffmpeg is not installed: no frames were taken")
		return
	}
	interval := 5.0
	if a.Duration > 0 {
		interval = math.Max(1, a.Duration/float64(maxFrames))
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	pat := filepath.Join(dir, "f%03d.jpg")
	cmd := exec.CommandContext(cctx, ffmpeg, "-nostdin", "-loglevel", "error", "-i", src, "-vf", "fps=1/"+strconv.FormatFloat(interval, 'f', 3, 64)+",scale='min(1280,iw)':-2",
		"-frames:v", strconv.Itoa(maxFrames), "-q:v", "3", pat)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		a.Notes = append(a.Notes, "ffmpeg could not read the video: "+strings.TrimSpace(firstLine(errb.String())))
	}
	files, _ := filepath.Glob(filepath.Join(dir, "f*.jpg"))
	sort.Strings(files)
	for i, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		img, err := media.AnalyseBytes(ctx, fmt.Sprintf("%s#frame%02d.jpg", a.Name, i+1), b, MaxImage)
		if err != nil {
			continue
		}
		fr := Frame{AtSeconds: math.Round(float64(i)*interval*10) / 10, URL: img.URL, SHA256: img.SHA256, DHash: img.DHashHex(), Meta: nil}
		if t := img.Meta["OCRText"]; t != "" {
			fr.OCR = t
			a.Text += fmt.Sprintf("[%.0fs] %s\n", fr.AtSeconds, t)
		}
		if img.HasGPS {
			fr.GPS = fmt.Sprintf("%.5f,%.5f", img.Lat, img.Lon)
		}
		a.Frames = append(a.Frames, fr)
	}
	if len(a.Frames) == 0 && len(a.Notes) == 0 {
		a.Notes = append(a.Notes, "no frames could be extracted")
	}
	if e, _ := media.OCRPath(); e == "" && len(a.Frames) > 0 {
		a.Notes = append(a.Notes, "OCR is off: install tesseract (or easyocr) to read text in the frames")
	}
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	return s
}

// readProbe fills the container metadata: duration, codecs, creation time, device and location tags.
func readProbe(ctx context.Context, probe, src string, a *Analysis) {
	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, probe, "-v", "quiet", "-print_format", "json", "-show_format", "-show_streams", src).Output()
	if err != nil {
		a.Notes = append(a.Notes, "ffprobe could not read the file")
		return
	}
	var p struct {
		Format struct {
			Duration string            `json:"duration"`
			Format   string            `json:"format_long_name"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Type   string            `json:"codec_type"`
			Codec  string            `json:"codec_name"`
			Width  int               `json:"width"`
			Height int               `json:"height"`
			Tags   map[string]string `json:"tags"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &p) != nil {
		return
	}
	if d, err := strconv.ParseFloat(p.Format.Duration, 64); err == nil {
		a.Duration = math.Round(d*10) / 10
		a.Meta["duration"] = strconv.FormatFloat(a.Duration, 'f', 1, 64) + " s"
	}
	if p.Format.Format != "" {
		a.Meta["container"] = p.Format.Format
	}
	for k, v := range p.Format.Tags {
		lk := strings.ToLower(k)
		switch {
		case lk == "creation_time", lk == "encoder", lk == "title", lk == "artist", lk == "comment", lk == "copyright", lk == "description",
			strings.Contains(lk, "location"), strings.Contains(lk, "make"), strings.Contains(lk, "model"), strings.Contains(lk, "software"), strings.Contains(lk, "author"):
			if len(v) < 300 {
				a.Meta[k] = v
			}
		}
	}
	for _, s := range p.Streams {
		if s.Type == "video" && s.Width > 0 {
			a.Meta["video"] = fmt.Sprintf("%s %dx%d", s.Codec, s.Width, s.Height)
		}
		if s.Type == "audio" {
			a.Meta["audio"] = s.Codec
		}
	}
	// "+30.0444+031.2357/" (ISO 6709) is how phones store the position
	for k, v := range a.Meta {
		if strings.Contains(strings.ToLower(k), "location") {
			if lat, lon, ok := parseISO6709(v); ok {
				a.Lat, a.Lon, a.GPS = lat, lon, fmt.Sprintf("%.5f,%.5f", lat, lon)
			}
		}
	}
}

func parseISO6709(s string) (lat, lon float64, ok bool) {
	s = strings.TrimSuffix(strings.TrimSpace(s), "/")
	var parts []string
	cur := ""
	for i, r := range s {
		if (r == '+' || r == '-') && i > 0 && cur != "" {
			parts = append(parts, cur)
			cur = ""
		}
		cur += string(r)
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	if len(parts) < 2 {
		return 0, 0, false
	}
	la, e1 := strconv.ParseFloat(parts[0], 64)
	lo, e2 := strconv.ParseFloat(parts[1], 64)
	if e1 != nil || e2 != nil || la < -90 || la > 90 || lo < -180 || lo > 180 {
		return 0, 0, false
	}
	return la, lo, true
}
