// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

// Package iddecode reads the creation time hidden inside platform identifiers.
// X/Twitter, Discord, TikTok, Instagram, Bluesky, LinkedIn and Mastodon all
// build their IDs from a timestamp, so a pasted post, video or profile link
// reveals when that item (or account) was created, without a single request.
// Everything here is offline arithmetic on the URL.
package iddecode

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

// Decoded is one identifier and the moment it encodes.
type Decoded struct {
	Platform string
	Kind     string // post, video, account, activity…
	ID       string
	At       time.Time
	URL      string
}

// Scanner is the module.
type Scanner struct{}

var _ scanner.Scanner = (*Scanner)(nil)

// New returns the module.
func New() *Scanner { return &Scanner{} }

func (s *Scanner) Name() string { return "iddecode" }
func (s *Scanner) Description() string {
	return "creation time hidden in post/video/account IDs of pasted links (X, Discord, TikTok, Instagram, Bluesky, LinkedIn, Mastodon)"
}

func (s *Scanner) CanRun(t models.Target) bool { return len(sources(t)) > 0 }

// sources returns every URL-looking string the operator supplied.
func sources(t models.Target) []string {
	var out []string
	seen := map[string]bool{}
	add := func(u string) {
		u = strings.TrimRight(strings.TrimSpace(u), `.,;)"'>]`)
		if u != "" && !seen[u] {
			seen[u] = true
			out = append(out, u)
		}
	}
	for _, l := range t.Links {
		add(l)
	}
	for _, m := range urlRe.FindAllString(t.Notes, 200) {
		add(m)
	}
	add(t.Domain)
	return out
}

var urlRe = regexp.MustCompile(`https?://[^\s<>"']+`)

func (s *Scanner) Scan(ctx context.Context, t models.Target, d scanner.Deps, emit func(models.Finding)) error {
	for _, u := range sources(t) {
		for _, dec := range Decode(u) {
			emit(finding(dec))
		}
	}
	return nil
}

func finding(d Decoded) models.Finding {
	age := time.Since(d.At)
	det := map[string]string{
		"platform": d.Platform, "kind": d.Kind, "id": d.ID,
		"created (UTC)": d.At.UTC().Format("2006-01-02 15:04:05"),
		"age":           humanAge(age),
		"note":          "decoded from the identifier itself, offline; it dates this " + d.Kind + ", not necessarily the person's first activity on the platform",
	}
	return models.Finding{Source: d.Platform + " ID timestamp", Category: "idinfo", URL: d.URL, Status: models.StatusFound, Confidence: 0.95, Details: det}
}

func humanAge(d time.Duration) string {
	if d < 0 {
		return "in the future (check the ID)"
	}
	days := int(d.Hours() / 24)
	switch {
	case days < 2:
		return strconv.Itoa(int(d.Hours())) + " hours"
	case days < 90:
		return strconv.Itoa(days) + " days"
	case days < 730:
		return strconv.Itoa(days/30) + " months"
	}
	return fmt.Sprintf("%.1f years", float64(days)/365.25)
}

// Epochs (milliseconds since the Unix epoch) of the snowflake-style schemes.
const (
	twitterEpoch   = 1288834974657
	discordEpoch   = 1420070400000
	instagramEpoch = 1314220021721
)

func fromSnowflake(id uint64, epoch int64, shift uint) time.Time {
	return time.UnixMilli(int64(id>>shift) + epoch).UTC()
}

var (
	reTweet     = regexp.MustCompile(`(?i)^/(?:[^/]+|i/web)/status(?:es)?/(\d{10,20})`)
	reTwitterID = regexp.MustCompile(`(?i)^/i/user/(\d{10,20})`)
	reDiscord   = regexp.MustCompile(`(?i)^/(?:users|channels/\d+|channels/@me)/(\d{15,21})`)
	reTikTok    = regexp.MustCompile(`(?i)^/@[^/]+/(?:video|photo)/(\d{15,21})`)
	reInstaPost = regexp.MustCompile(`(?i)^/(?:[^/]+/)?(?:p|reel|reels|tv)/([A-Za-z0-9_-]{6,})`)
	reBsky      = regexp.MustCompile(`(?i)^/profile/[^/]+/post/([2-7a-z]{13})$`)
	reMastodon  = regexp.MustCompile(`^/@[^/]+/(\d{17,20})$`)
	reLinkedIn  = regexp.MustCompile(`(?i)(?:activity|ugcPost|share)[-:](\d{18,20})`)
)

// Decode returns every timestamp the URL's identifiers carry.
func Decode(raw string) []Decoded {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	path := u.Path
	var out []Decoded
	add := func(platform, kind, id string, at time.Time) {
		if at.Year() >= 2000 && at.Before(time.Now().AddDate(1, 0, 0)) {
			out = append(out, Decoded{Platform: platform, Kind: kind, ID: id, At: at, URL: raw})
		}
	}
	parse := func(s string) (uint64, bool) { v, err := strconv.ParseUint(s, 10, 64); return v, err == nil }

	switch {
	case host == "x.com" || host == "twitter.com" || host == "mobile.twitter.com":
		if m := reTweet.FindStringSubmatch(path); m != nil {
			if id, ok := parse(m[1]); ok && id > 1<<22 {
				add("X/Twitter", "post", m[1], fromSnowflake(id, twitterEpoch, 22))
			}
		}
		if m := reTwitterID.FindStringSubmatch(path); m != nil {
			if id, ok := parse(m[1]); ok && id > 1<<30 { // accounts made before Nov 2010 have small sequential ids
				add("X/Twitter", "account", m[1], fromSnowflake(id, twitterEpoch, 22))
			}
		}
	case host == "discord.com" || host == "discordapp.com" || host == "discord.gg":
		if m := reDiscord.FindStringSubmatch(path); m != nil {
			if id, ok := parse(m[1]); ok {
				add("Discord", "account/server/channel", m[1], fromSnowflake(id, discordEpoch, 22))
			}
		}
		for _, m := range regexp.MustCompile(`/(\d{17,20})`).FindAllStringSubmatch(path, 4) { // message ids
			if id, ok := parse(m[1]); ok && !contains(out, m[1]) {
				add("Discord", "message/channel", m[1], fromSnowflake(id, discordEpoch, 22))
			}
		}
	case strings.HasSuffix(host, "tiktok.com"):
		if m := reTikTok.FindStringSubmatch(path); m != nil {
			if id, ok := parse(m[1]); ok {
				add("TikTok", "video", m[1], time.Unix(int64(id>>32), 0).UTC())
			}
		}
	case host == "instagram.com":
		if m := reInstaPost.FindStringSubmatch(path); m != nil {
			if id, ok := instagramID(m[1]); ok {
				add("Instagram", "post", m[1], fromSnowflake(id, instagramEpoch, 23))
			}
		}
	case host == "bsky.app":
		if m := reBsky.FindStringSubmatch(path); m != nil {
			if at, ok := tidTime(m[1]); ok {
				add("Bluesky", "post", m[1], at)
			}
		}
	case strings.HasSuffix(host, "linkedin.com"):
		if m := reLinkedIn.FindStringSubmatch(raw); m != nil {
			if id, ok := parse(m[1]); ok {
				add("LinkedIn", "post/activity", m[1], time.UnixMilli(int64(id>>22)).UTC())
			}
		}
	}
	// Mastodon-style hosts: /@user/<id> where the id embeds milliseconds in its top 48 bits.
	if len(out) == 0 {
		if m := reMastodon.FindStringSubmatch(path); m != nil {
			if id, ok := parse(m[1]); ok {
				add("Mastodon ("+host+")", "post", m[1], time.UnixMilli(int64(id>>16)).UTC())
			}
		}
	}
	return out
}

func contains(ds []Decoded, id string) bool {
	for _, d := range ds {
		if d.ID == id {
			return true
		}
	}
	return false
}

const instaAlphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// instagramID turns a post shortcode into its numeric media id (base 64).
func instagramID(code string) (uint64, bool) {
	if len(code) > 11 { // private posts append a key; only the first 11 characters are the media id
		code = code[:11]
	}
	v := new(big.Int)
	for _, r := range code {
		i := strings.IndexRune(instaAlphabet, r)
		if i < 0 {
			return 0, false
		}
		v.Mul(v, big.NewInt(64)).Add(v, big.NewInt(int64(i)))
	}
	if !v.IsUint64() {
		return 0, false
	}
	return v.Uint64(), true
}

const tidAlphabet = "234567abcdefghijklmnopqrstuvwxyz"

// tidTime decodes an AT Protocol record key (TID): 13 base32-sortable
// characters holding microseconds since the epoch in bits 10 and up.
func tidTime(tid string) (time.Time, bool) {
	if len(tid) != 13 {
		return time.Time{}, false
	}
	var v uint64
	for _, r := range tid {
		i := strings.IndexRune(tidAlphabet, r)
		if i < 0 {
			return time.Time{}, false
		}
		v = v<<5 | uint64(i)
	}
	if v>>63 != 0 {
		return time.Time{}, false
	}
	return time.UnixMicro(int64(v >> 10)).UTC(), true
}
