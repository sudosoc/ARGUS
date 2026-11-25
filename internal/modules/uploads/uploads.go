// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package uploads turns files the operator supplied (pictures, documents, videos) into findings.
// The files were analysed locally when they were uploaded (package files); nothing is fetched.
package uploads

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sudosoc/argus/internal/files"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "uploads" }
func (s *Scanner) Description() string {
	return "files you supplied: picture EXIF/OCR, document metadata and text, video frames; leads found inside them"
}
func (s *Scanner) CanRun(t models.Target) bool { return len(t.Files) > 0 }

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	for _, id := range t.Files {
		a := files.Lookup(id)
		if a == nil {
			emit(models.Finding{Source: "upload " + id, Category: "file", Status: models.StatusError,
				Details: map[string]string{"error": "the uploaded file is no longer available on the server (re-upload it)"}})
			continue
		}
		emit(finding(a))
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func finding(a *files.Analysis) models.Finding {
	d := map[string]string{
		"file": a.Name, "type": a.Kind, "size": strconv.Itoa(a.Size) + " bytes", "sha256": a.SHA256,
	}
	keys := make([]string, 0, len(a.Meta))
	for k := range a.Meta {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		d[k] = a.Meta[k]
	}
	if a.Width > 0 {
		d["dimensions"] = fmt.Sprintf("%d x %d", a.Width, a.Height)
	}
	if a.Camera != "" {
		d["camera"] = a.Camera
	}
	if a.Taken != "" {
		d["taken"] = a.Taken
	}
	if a.GPS != "" {
		d["gps"] = a.GPS
	}
	if a.DHash != "" {
		d["perceptual hash"] = a.DHash
	}
	if a.Duration > 0 {
		d["duration"] = strconv.FormatFloat(a.Duration, 'f', 1, 64) + " s"
	}
	if a.TextChars > 0 {
		d["text characters"] = strconv.Itoa(a.TextChars)
		d["text"] = a.Text
	}
	if a.Name_ != "" {
		d["name in the text"] = a.Name_
	}
	if len(a.Employers) > 0 {
		d["employers in the text"] = strings.Join(a.Employers, "; ")
	}
	if len(a.Notes) > 0 {
		d["note"] = strings.Join(a.Notes, "\n")
	}
	for i, f := range a.Frames {
		line := fmt.Sprintf("at %.0f s, sha256 %s", f.AtSeconds, f.SHA256[:12])
		if f.OCR != "" {
			line += ", text: " + strings.ReplaceAll(f.OCR, "\n", " / ")
		}
		d[fmt.Sprintf("frame %02d", i+1)] = line
	}
	for k, list := range map[string][]string{"e-mails": a.Emails, "phones": a.Phones, "domains": a.Domains, "wallets": a.Wallets} {
		if len(list) > 0 {
			d[k+" found inside"] = strings.Join(list, "\n")
		}
	}
	f := models.Finding{Source: "upload: " + a.Name, Category: "file", Status: models.StatusFound, Confidence: 0.9, Details: d}
	if a.ImageURL != "" {
		f.URL = a.ImageURL
		f.Entities = append(f.Entities, models.Entity{Type: models.EntImage, Value: a.ImageURL})
	}
	for _, fr := range a.Frames {
		f.Entities = append(f.Entities, models.Entity{Type: models.EntImage, Value: fr.URL})
	}
	for _, e := range a.Emails {
		f.Entities = append(f.Entities, models.Entity{Type: models.EntEmail, Value: e})
	}
	for _, p := range a.Phones {
		f.Entities = append(f.Entities, models.Entity{Type: models.EntPhone, Value: p})
	}
	for _, dom := range a.Domains {
		f.Entities = append(f.Entities, models.Entity{Type: models.EntDomain, Value: dom})
	}
	if a.Name_ != "" {
		f.Entities = append(f.Entities, models.Entity{Type: models.EntName, Value: a.Name_})
	}
	for _, o := range a.Employers {
		f.Entities = append(f.Entities, models.Entity{Type: models.EntOrg, Value: o})
	}
	if a.Location != "" {
		f.Entities = append(f.Entities, models.Entity{Type: models.EntLocation, Value: a.Location})
	}
	return f
}
