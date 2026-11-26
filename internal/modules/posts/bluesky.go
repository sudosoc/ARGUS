// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package posts

import (
	"context"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/util"
)

var blueskyAPI = "https://public.api.bsky.app/xrpc"

var bskyMention = regexp.MustCompile(`@([A-Za-z0-9][A-Za-z0-9.-]*\.[A-Za-z]{2,})`)

type bItem struct {
	Post struct {
		URI    string `json:"uri"`
		Author struct {
			Handle string `json:"handle"`
		} `json:"author"`
		Record struct {
			Text    string    `json:"text"`
			Created time.Time `json:"createdAt"`
			Langs   []string  `json:"langs"`
			Facets  []struct {
				Features []struct {
					Type string `json:"$type"`
					URI  string `json:"uri"`
					Tag  string `json:"tag"`
				} `json:"features"`
			} `json:"facets"`
		} `json:"record"`
		Embed struct {
			Images []struct {
				Fullsize string `json:"fullsize"`
				Thumb    string `json:"thumb"`
			} `json:"images"`
			External struct {
				URI string `json:"uri"`
			} `json:"external"`
		} `json:"embed"`
		Likes   int `json:"likeCount"`
		Reposts int `json:"repostCount"`
		Replies int `json:"replyCount"`
	} `json:"post"`
	Reply struct {
		Parent struct {
			Author struct {
				Handle string `json:"handle"`
			} `json:"author"`
		} `json:"parent"`
	} `json:"reply"`
	Reason *struct {
		Type string `json:"$type"`
	} `json:"reason"`
}

func bluesky(ctx context.Context, c *httpx.Client, handle string) ([]models.Post, string, string) {
	h := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(handle), "@"))
	if !strings.Contains(h, ".") {
		h += ".bsky.social"
	}
	sp := &spacer{}
	sp.wait(ctx)
	var prof struct {
		DID    string `json:"did"`
		Handle string `json:"handle"`
		Labels []struct {
			Val string `json:"val"`
		} `json:"labels"`
	}
	code, ok := util.GetJSONStatus(ctx, c, blueskyAPI+"/app.bsky.actor.getProfile?actor="+url.QueryEscape(h), &prof)
	if code == 429 || code == 403 {
		return nil, "", "Bluesky refused the request (HTTP " + itoa(code) + "): not collected"
	}
	if !ok || prof.DID == "" {
		return nil, "", "no Bluesky account with this name"
	}
	profileURL := "https://bsky.app/profile/" + prof.Handle
	for _, l := range prof.Labels {
		if l.Val == "!no-unauthenticated" { // the account asked not to be shown to people who are not signed in
			return nil, profileURL, "the account asked not to be shown to people without an account: not collected"
		}
	}
	var out []models.Post
	cursor := ""
	for page := 0; page < 2 && len(out) < maxPosts; page++ {
		if !sp.wait(ctx) {
			break
		}
		u := blueskyAPI + "/app.bsky.feed.getAuthorFeed?limit=100&filter=posts_with_replies&actor=" + url.QueryEscape(prof.DID)
		if cursor != "" {
			u += "&cursor=" + url.QueryEscape(cursor)
		}
		var r struct {
			Feed   []bItem `json:"feed"`
			Cursor string  `json:"cursor"`
		}
		code, ok := util.GetJSONStatus(ctx, c, u, &r)
		if !ok {
			if code == 429 || code == 403 || code == 401 {
				return out, profileURL, "Bluesky stopped showing the feed without signing in (HTTP " + itoa(code) + "); partial"
			}
			break
		}
		for _, it := range r.Feed {
			out = append(out, bskyPost(it, prof.Handle))
		}
		if r.Cursor == "" || len(r.Feed) == 0 {
			break
		}
		cursor = r.Cursor
	}
	return out, profileURL, ""
}

func bskyPost(it bItem, handle string) models.Post {
	rec := it.Post.Record
	kind, author := "post", ""
	switch {
	case it.Reason != nil && strings.HasSuffix(it.Reason.Type, "reasonRepost"):
		kind, author = "boost", it.Post.Author.Handle
	case it.Reply.Parent.Author.Handle != "":
		kind = "reply"
	}
	rkey := it.Post.URI
	if i := strings.LastIndex(rkey, "/"); i >= 0 {
		rkey = rkey[i+1:]
	}
	p := models.Post{Platform: "Bluesky", Handle: handle, ID: rkey, URL: "https://bsky.app/profile/" + it.Post.Author.Handle + "/post/" + rkey,
		Time: utc(rec.Created), Kind: kind, Text: clip(rec.Text, maxText), Author: author, ReplyTo: it.Reply.Parent.Author.Handle,
		Likes: it.Post.Likes, Reposts: it.Post.Reposts, Replies: it.Post.Replies}
	if len(rec.Langs) > 0 {
		p.Lang = strings.ToLower(rec.Langs[0])
	}
	for _, f := range rec.Facets {
		for _, ft := range f.Features {
			switch {
			case strings.HasSuffix(ft.Type, "#tag") && ft.Tag != "":
				p.Tags = append(p.Tags, strings.ToLower(ft.Tag))
			case strings.HasSuffix(ft.Type, "#link") && ft.URI != "":
				p.Links = append(p.Links, ft.URI)
			}
		}
	}
	for _, m := range bskyMention.FindAllStringSubmatch(rec.Text, -1) {
		p.Mentions = append(p.Mentions, strings.ToLower(m[1]))
	}
	if it.Post.Embed.External.URI != "" {
		p.Links = append(p.Links, it.Post.Embed.External.URI)
	}
	for _, im := range it.Post.Embed.Images {
		if im.Fullsize != "" && len(p.Media) < maxMedia {
			p.Media = append(p.Media, im.Fullsize)
		}
	}
	p.Tags, p.Mentions, p.Links = uniq(p.Tags), uniq(p.Mentions), uniq(p.Links)
	return p
}
