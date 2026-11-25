// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package web

import (
	"io"
	"net/http"
	"net/url"

	"github.com/sudosoc/argus/internal/files"
)

// handleUpload analyses one uploaded file (the body is the raw file, its name is in the
// X-Filename header, URL-escaped) and returns what was found in it. Nothing leaves the machine.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	name, _ := url.QueryUnescape(r.Header.Get("X-Filename"))
	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusRequestEntityTooLarge, "the file is too large")
		return
	}
	a, err := files.Analyze(r.Context(), name, data)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, a)
}
