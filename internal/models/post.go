// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package models

// Post is one public item an account published (a post, a comment, a commit message, a repository),
// copied as it was publicly visible to anyone without an account. It is evidence, kept with the case.
type Post struct {
	Platform string   `json:"platform"` // Mastodon | Bluesky | GitHub | Reddit
	Handle   string   `json:"handle"`   // the account it was collected from
	ID       string   `json:"id,omitempty"`
	URL      string   `json:"url,omitempty"`
	Time     string   `json:"time,omitempty"` // RFC 3339, UTC
	Kind     string   `json:"kind"`           // post | reply | boost | comment | commit | issue | pull | repo | star | gist
	Text     string   `json:"text,omitempty"` // capped; the author's own words (or the boosted post's)
	Lang     string   `json:"lang,omitempty"`
	Author   string   `json:"author,omitempty"` // the original author when Kind is boost
	ReplyTo  string   `json:"reply_to,omitempty"`
	Tags     []string `json:"tags,omitempty"`     // hashtags, subreddits, repository topics
	Mentions []string `json:"mentions,omitempty"` // accounts named in the text
	Links    []string `json:"links,omitempty"`
	Media    []string `json:"media,omitempty"` // picture addresses (never downloaded unless the picture stage asks)
	Likes    int      `json:"likes,omitempty"`
	Reposts  int      `json:"reposts,omitempty"`
	Replies  int      `json:"replies,omitempty"`
}

// PostSource says what was collected from one account and why a collection may be partial or empty.
type PostSource struct {
	Platform string `json:"platform"`
	Handle   string `json:"handle"`
	URL      string `json:"url,omitempty"`
	Count    int    `json:"count"`
	Oldest   string `json:"oldest,omitempty"`
	Newest   string `json:"newest,omitempty"`
	Note     string `json:"note,omitempty"` // for example "the account asked not to be shown to people without an account: not collected"
}
