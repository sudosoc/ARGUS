package media

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var (
	exifOnce sync.Once
	exifBin  string
	exifSem  = make(chan struct{}, 4)
)

// ExifToolPath returns the exiftool binary if it is installed.
func ExifToolPath() string {
	exifOnce.Do(func() { exifBin, _ = exec.LookPath("exiftool") })
	return exifBin
}

// exifKeep lists the tags worth surfacing, in display order. Platforms strip
// most of them from avatars, but personal sites, CV photos and shared images
// often keep device, author and software traces.
var exifKeep = []string{"Make", "Model", "LensModel", "Software", "CreatorTool", "Artist", "Creator", "By-line", "OwnerName",
	"CameraOwnerName", "Copyright", "Rights", "SerialNumber", "BodySerialNumber", "LensSerialNumber", "InternalSerialNumber",
	"DateTimeOriginal", "CreateDate", "ModifyDate", "OffsetTimeOriginal", "GPSAltitude", "ImageUniqueID", "Caption-Abstract",
	"Description", "Title", "XPAuthor", "HostComputer", "Warning"}

// readExifTool fills what the built-in reader cannot: dozens of formats, XMP /
// IPTC authorship, serial numbers (which link two pictures to one camera) and
// GPS in more containers. It is a no-op when exiftool is not installed.
func readExifTool(ctx context.Context, img *Image) {
	bin := ExifToolPath()
	if bin == "" || len(img.Data) < 64 || img.ContentType == "image/svg+xml" {
		return
	}
	select {
	case exifSem <- struct{}{}:
		defer func() { <-exifSem }()
	case <-ctx.Done():
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-json", "-n", "-G0", "-q", "-fast2", "-ignoreMinorErrors", "-")
	cmd.Stdin = bytes.NewReader(img.Data)
	var out bytes.Buffer
	cmd.Stdout = &out
	if cmd.Run() != nil && out.Len() == 0 {
		return
	}
	parseExifTool(out.Bytes(), img)
}

// parseExifTool reads exiftool's -json -G0 output into img.
func parseExifTool(raw []byte, img *Image) {
	var rows []map[string]any
	if json.Unmarshal(raw, &rows) != nil || len(rows) == 0 {
		return
	}
	row := rows[0]
	byTag := map[string]string{}
	for k, v := range row {
		_, tag, ok := strings.Cut(k, ":")
		if !ok {
			tag = k
		}
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" || s == "<nil>" || len(s) > 200 {
			continue
		}
		if _, dup := byTag[tag]; !dup {
			byTag[tag] = s
		}
	}
	meta := map[string]string{}
	for _, tag := range exifKeep {
		if v, ok := byTag[tag]; ok {
			meta[tag] = v
		}
	}
	if len(meta) > 0 {
		img.Meta = meta
	}
	if !img.HasGPS {
		lat, lon := num(row, "GPSLatitude"), num(row, "GPSLongitude")
		if (lat != 0 || lon != 0) && lat >= -90 && lat <= 90 && lon >= -180 && lon <= 180 {
			img.HasGPS, img.Lat, img.Lon = true, lat, lon
		}
	}
	if img.Taken == "" {
		if t := byTag["DateTimeOriginal"]; t != "" {
			img.Taken = t
		} else if t := byTag["CreateDate"]; t != "" {
			img.Taken = t
		}
	}
	if img.Camera == "" {
		img.Camera = strings.TrimSpace(byTag["Make"] + " " + byTag["Model"])
	}
}

// num finds a numeric tag under any group prefix ("EXIF:GPSLatitude", "Composite:GPSLatitude").
func num(row map[string]any, tag string) float64 {
	for k, v := range row {
		if k == tag || strings.HasSuffix(k, ":"+tag) {
			if f, ok := v.(float64); ok {
				return f
			}
		}
	}
	return 0
}
