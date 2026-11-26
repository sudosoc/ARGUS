// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package posts

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/media"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/util"
)

var mastodonHosts = []string{"mastodon.social", "mstdn.social", "fosstodon.org", "infosec.exchange", "hachyderm.io", "mas.to"}

// mastodonBase is the address of an instance's API; tests replace it.
var mastodonBase = func(host string) string { return "https://" + host }

type mStatus struct {
	ID         string    `json:"id"`
	Created    time.Time `json:"created_at"`
	InReplyTo  string    `json:"in_reply_to_id"`
	Spoiler    string    `json:"spoiler_text"`
	Language   string    `json:"language"`
	URL        string    `json:"url"`
	Replies    int       `json:"replies_count"`
	Reblogs    int       `json:"reblogs_count"`
	Favourites int       `json:"favourites_count"`
	Content    string    `json:"content"`
	Visibility string    `json:"visibility"`
	Reblog     *mStatus  `json:"reblog"`
	Account    struct {
		Acct string `json:"acct"`
	} `json:"account"`
	Attachments []struct {
		Type    string `json:"type"`
		URL     string `json:"url"`
		Preview string `json:"preview_url"`
	} `json:"media_attachments"`
	Mentions []struct {
		Acct string `json:"acct"`
	} `json:"mentions"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

func mastodon(ctx context.Context, c *httpx.Client, handle string) ([]models.Post, string, string) {
	name, host := handle, ""
	if i := strings.Index(handle, "@"); i > 0 {
		name, host = handle[:i], strings.ToLower(handle[i+1:])
	}
	hosts := mastodonHosts
	if host != "" {
		if !strings.Contains(host, ".") || media.CheckURL(ctx, "https://"+host) != nil {
			return nil, "", "that instance name could not be used"
		}
		hosts = []string{host}
	}
	sp := &spacer{}
	for _, h := range hosts {
		if !sp.wait(ctx) {
			return nil, "", ""
		}
		var acct struct {
			ID      string `json:"id"`
			URL     string `json:"url"`
			Noindex bool   `json:"noindex"`
		}
		status, ok := util.GetJSONStatus(ctx, c, mastodonBase(h)+"/api/v1/accounts/lookup?acct="+url.QueryEscape(name), &acct)
		if status == 429 || status == 403 {
			return nil, "", h + " refused the request (HTTP " + itoa(status) + "): not collected"
		}
		if !ok || acct.ID == "" {
			continue
		}
		if acct.Noindex {
			return nil, acct.URL, "the account asked not to be indexed (noindex): not collected"
		}
		var out []models.Post
		seen := map[string]bool{}
		maxID := ""
		for page := 0; page < 4 && len(out) < maxPosts; page++ {
			if !sp.wait(ctx) {
				break
			}
			u := mastodonBase(h) + "/api/v1/accounts/" + acct.ID + "/statuses?limit=40"
			if maxID != "" {
				u += "&max_id=" + url.QueryEscape(maxID)
			}
			var st []mStatus
			code, ok := util.GetJSONStatus(ctx, c, u, &st)
			if !ok {
				if code == 429 || code == 403 || code == 401 {
					return out, acct.URL, h + " stopped showing the account's posts without signing in (HTTP " + itoa(code) + "); partial"
				}
				break
			}
			if len(st) == 0 {
				break
			}
			fresh := 0
			for _, s := range st {
				if seen[s.ID] { // a server that ignores the page marker repeats itself
					continue
				}
				seen[s.ID] = true
				fresh++
				if s.Visibility != "" && s.Visibility != "public" && s.Visibility != "unlisted" {
					continue
				}
				out = append(out, mastodonPost(s, name+"@"+h))
				maxID = s.ID
			}
			if fresh == 0 {
				break
			}
		}
		return out, acct.URL, ""
	}
	return nil, "", "no account with this name on the largest Mastodon servers (name@server finds one elsewhere)"
}

func mastodonPost(s mStatus, handle string) models.Post {
	src, kind, author := s, "post", ""
	if s.Reblog != nil {
		src, kind, author = *s.Reblog, "boost", s.Reblog.Account.Acct
	} else if s.InReplyTo != "" {
		kind = "reply"
	}
	text := plain(src.Content)
	if src.Spoiler != "" {
		text = "[" + src.Spoiler + "] " + text
	}
	p := models.Post{Platform: "Mastodon", Handle: handle, ID: s.ID, URL: s.URL, Time: utc(s.Created), Kind: kind, Text: clip(text, maxText),
		Lang: src.Language, Author: author, Likes: src.Favourites, Reposts: src.Reblogs, Replies: src.Replies, Links: links(text)}
	for _, t := range src.Tags {
		p.Tags = append(p.Tags, strings.ToLower(t.Name))
	}
	for _, m := range src.Mentions {
		p.Mentions = append(p.Mentions, m.Acct)
	}
	for _, a := range src.Attachments {
		u := a.URL
		if a.Type != "image" {
			u = a.Preview
		}
		if u != "" && len(p.Media) < maxMedia {
			p.Media = append(p.Media, u)
		}
	}
	p.Tags, p.Mentions = uniq(p.Tags), uniq(p.Mentions)
	return p
}
