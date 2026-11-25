package web

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/sudosoc/argus/internal/settings"
)

// The settings API never returns a key: only whether one exists, where it comes from and its last four characters.

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"providers": s.settings.Views(), "llm": s.settings.LLM(),
		"storage": "encrypted file in your config directory; environment variables are also read"})
}

type settingsReq struct {
	Enabled *bool   `json:"enabled"`
	Key     *string `json:"key"` // absent: keep; "": clear
	Model   *string `json:"model"`
	BaseURL *string `json:"base_url"`
}

func (s *Server) handleSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	var req settingsReq
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	if err := s.settings.Update(r.PathValue("id"), req.Enabled, req.Key, req.Model, req.BaseURL); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.handleSettings(w, r)
}

func (s *Server) handleSettingsLLM(w http.ResponseWriter, r *http.Request) {
	var c settings.LLMChoice
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	c.Model = strings.TrimSpace(c.Model)
	if len(c.Model) > 120 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "model name is too long"})
		return
	}
	if err := s.settings.SetLLM(c); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.handleSettings(w, r)
}

// handleSettingsTest pings the provider with the key typed in the form (not yet saved) or the stored one.
func (s *Server) handleSettingsTest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p, ok := settings.ProviderByID(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown provider"})
		return
	}
	var req settingsReq
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req)
	key, _ := s.settings.Resolve(id)
	if req.Key != nil && strings.TrimSpace(*req.Key) != "" {
		key = strings.TrimSpace(*req.Key)
	}
	e := s.settings.Entry(id)
	model, base := e.Model, e.BaseURL
	if req.Model != nil {
		model = strings.TrimSpace(*req.Model)
	}
	if req.BaseURL != nil {
		base = strings.TrimSpace(*req.BaseURL)
	}
	if p.Kind != "llm" {
		// Tool keys have no universal ping; report that the key is present rather than pretending.
		if key == "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": "no key set"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "a key is set; this key is used only by the tools that need it, so it cannot be checked here"})
		return
	}
	if err := settings.TestLLMConnectionAt(r.Context(), id, key, model, base); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "connected"})
}
