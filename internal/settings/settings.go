// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package settings stores API keys and the LLM choice. Keys live in an
// AES-256-GCM encrypted file under the user's config directory, never in the
// front end, never in reports or cases. Environment variables are also read,
// and win when no key was stored.
//
// The encryption key is a random 32-byte file next to the settings (0600), or
// the value of ARGUS_SETTINGS_KEY when set. This protects the file from being
// read by accident (backups, a stray commit, a copied directory); it does not
// protect against someone who can already read the key file as the same user.
package settings

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Provider describes one API the tool can use.
type Provider struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Kind         string `json:"kind"`    // "llm" or "tool"
	EnvVar       string `json:"env_var"` // variable tools receive, and the one read as a fallback
	NeedsKey     bool   `json:"needs_key"`
	DefaultModel string `json:"default_model,omitempty"`
	DefaultBase  string `json:"default_base,omitempty"`
	Help         string `json:"help,omitempty"`
}

// Providers is the fixed list shown on the settings page.
var Providers = []Provider{
	{ID: "anthropic", Label: "Anthropic (Claude)", Kind: "llm", EnvVar: "ANTHROPIC_API_KEY", NeedsKey: true, DefaultModel: "claude-sonnet-5-5", Help: "console.anthropic.com → API keys"},
	{ID: "openai", Label: "OpenAI", Kind: "llm", EnvVar: "OPENAI_API_KEY", NeedsKey: true, DefaultModel: "gpt-4o-mini", Help: "platform.openai.com → API keys"},
	{ID: "gemini", Label: "Google Gemini", Kind: "llm", EnvVar: "GEMINI_API_KEY", NeedsKey: true, DefaultModel: "gemini-1.5-flash", Help: "aistudio.google.com → Get API key"},
	{ID: "local", Label: "Local model (Ollama or any OpenAI-compatible server)", Kind: "llm", EnvVar: "", NeedsKey: false, DefaultModel: "llama3", DefaultBase: "http://127.0.0.1:11434", Help: "no key; give the server address"},
	{ID: "google", Label: "Google API key (used by The 3rd Eye)", Kind: "tool", EnvVar: "GOOGLE_API_KEY", NeedsKey: true, Help: "console.cloud.google.com → Credentials"},
}

// ProviderByID looks a provider up.
func ProviderByID(id string) (Provider, bool) {
	for _, p := range Providers {
		if p.ID == id {
			return p, true
		}
	}
	return Provider{}, false
}

// Entry is what is stored for one provider.
type Entry struct {
	Enabled bool   `json:"enabled"`
	Key     string `json:"key,omitempty"`
	Model   string `json:"model,omitempty"`
	BaseURL string `json:"base_url,omitempty"`
}

// LLMChoice is the selected provider and exact model name.
type LLMChoice struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type fileData struct {
	Entries map[string]Entry `json:"entries"`
	LLM     LLMChoice        `json:"llm"`
}

// Store reads and writes the encrypted settings file. The zero value is not usable; call Open.
type Store struct {
	mu   sync.Mutex
	path string
	key  []byte
	data fileData
	env  func(string) string
}

// DefaultDir is where settings live: $ARGUS_SETTINGS_DIR or <user config dir>/argus.
func DefaultDir() string {
	if d := os.Getenv("ARGUS_SETTINGS_DIR"); d != "" {
		return d
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "argus")
	}
	return ""
}

// Open loads the store from dir (created 0700). An empty dir gives an in-memory store.
func Open(dir string) (*Store, error) {
	s := &Store{env: os.Getenv, data: fileData{Entries: map[string]Entry{}}}
	if dir == "" {
		return s, nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	s.path = filepath.Join(dir, "settings.enc")
	key, err := loadKey(dir)
	if err != nil {
		return nil, err
	}
	s.key = key
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	plain, err := decrypt(key, raw)
	if err != nil {
		return nil, errors.New("settings file could not be decrypted (wrong ARGUS_SETTINGS_KEY or a damaged file)")
	}
	if err := json.Unmarshal(plain, &s.data); err != nil {
		return nil, err
	}
	if s.data.Entries == nil {
		s.data.Entries = map[string]Entry{}
	}
	return s, nil
}

func loadKey(dir string) ([]byte, error) {
	if v := os.Getenv("ARGUS_SETTINGS_KEY"); v != "" {
		sum := sha256.Sum256([]byte(v))
		return sum[:], nil
	}
	p := filepath.Join(dir, "settings.key")
	if b, err := os.ReadFile(p); err == nil {
		if k, err := hex.DecodeString(strings.TrimSpace(string(b))); err == nil && len(k) == 32 {
			return k, nil
		}
		return nil, errors.New("settings.key is damaged")
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	if err := os.WriteFile(p, []byte(hex.EncodeToString(k)), 0o600); err != nil {
		return nil, err
	}
	return k, nil
}

func encrypt(key, plain []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, g.Seal(nil, nonce, plain, []byte("argus-settings-v1"))...), nil
}

func decrypt(key, raw []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(blk)
	if err != nil {
		return nil, err
	}
	if len(raw) < g.NonceSize() {
		return nil, errors.New("short")
	}
	return g.Open(nil, raw[:g.NonceSize()], raw[g.NonceSize():], []byte("argus-settings-v1"))
}

func (s *Store) save() error {
	if s.path == "" {
		return nil
	}
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	enc, err := encrypt(s.key, b)
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, enc, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

// Update changes one provider. A nil key leaves the stored key as it is; an empty string clears it.
func (s *Store) Update(id string, enabled *bool, key *string, model, baseURL *string) error {
	p, ok := ProviderByID(id)
	if !ok {
		return errors.New("unknown provider")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.data.Entries[p.ID]
	if enabled != nil {
		e.Enabled = *enabled
	}
	if key != nil {
		k := strings.TrimSpace(*key)
		if len(k) > 400 || strings.ContainsAny(k, "\r\n\x00") {
			return errors.New("that does not look like an API key")
		}
		e.Key = k
	}
	if model != nil {
		e.Model = strings.TrimSpace(*model)
	}
	if baseURL != nil {
		e.BaseURL = strings.TrimSpace(*baseURL)
	}
	s.data.Entries[p.ID] = e
	return s.save()
}

// SetLLM records the chosen provider and model.
func (s *Store) SetLLM(c LLMChoice) error {
	if _, ok := ProviderByID(c.Provider); !ok {
		return errors.New("unknown provider")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.LLM = c
	return s.save()
}

// View is one provider as the front end sees it: the key itself never leaves the server.
type View struct {
	Provider
	Enabled   bool   `json:"enabled"`
	HasKey    bool   `json:"has_key"`
	KeyHint   string `json:"key_hint,omitempty"`
	KeySource string `json:"key_source,omitempty"` // "stored" or "env"
	Model     string `json:"model,omitempty"`
	BaseURL   string `json:"base_url,omitempty"`
}

func hint(k string) string {
	if len(k) <= 8 {
		return "••••"
	}
	return "••••" + k[len(k)-4:]
}

// Views lists every provider, masked.
func (s *Store) Views() []View {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]View, 0, len(Providers))
	for _, p := range Providers {
		e := s.data.Entries[p.ID]
		v := View{Provider: p, Enabled: e.Enabled, Model: e.Model, BaseURL: e.BaseURL}
		switch {
		case e.Key != "":
			v.HasKey, v.KeyHint, v.KeySource = true, hint(e.Key), "stored"
		case p.EnvVar != "" && s.env(p.EnvVar) != "":
			v.HasKey, v.KeyHint, v.KeySource = true, hint(s.env(p.EnvVar)), "env"
		}
		out = append(out, v)
	}
	return out
}

// LLM returns the chosen provider and model.
func (s *Store) LLM() LLMChoice {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.LLM
}

// Resolve returns the key to use for a provider and whether the provider is enabled.
// A stored key wins over the environment variable.
func (s *Store) Resolve(id string) (key string, enabled bool) {
	p, ok := ProviderByID(id)
	if !ok {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.data.Entries[id]
	key = e.Key
	if key == "" && p.EnvVar != "" {
		key = s.env(p.EnvVar)
	}
	return key, e.Enabled
}

// Entry returns the stored entry (including the key) for server-side use only.
func (s *Store) Entry(id string) Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Entries[id]
}

// IDs lists provider ids in display order.
func IDs() []string {
	ids := make([]string, 0, len(Providers))
	for _, p := range Providers {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	return ids
}
