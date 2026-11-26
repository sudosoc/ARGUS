// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package posts

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/util"
)

var redditBase = "https://www.reddit.com"

func reddit(ctx context.Context, c *httpx.Client, handle string) ([]models.Post, string, string) {
	name := strings.TrimPrefix(strings.TrimPrefix(handle, "@"), "u/")
	profileURL := "https://www.reddit.com/user/" + name
	sp := &spacer{}
	var out []models.Post
	adult := 0
	for _, kind := range []string{"submitted", "comments"} {
		sp.wait(ctx)
		var r struct {
			Data struct {
				Children []struct {
					Data struct {
						ID        string  `json:"id"`
						Created   float64 `json:"created_utc"`
						Subreddit string  `json:"subreddit"`
						Permalink string  `json:"permalink"`
						Title     string  `json:"title"`
						SelfText  string  `json:"selftext"`
						Body      string  `json:"body"`
						Score     int     `json:"score"`
						Comments  int     `json:"num_comments"`
						Adult     bool    `json:"over_18"`
						URL       string  `json:"url"`
					} `json:"data"`
				} `json:"children"`
			} `json:"data"`
		}
		code, ok := util.GetJSONStatus(ctx, c, redditBase+"/user/"+url.PathEscape(name)+"/"+kind+".json?limit=100&raw_json=1", &r)
		if !ok {
			switch {
			case code == 404:
				return nil, "", "no Reddit account with this name (or it was suspended)"
			case code == 401 || code == 403 || code == 429:
				if len(out) > 0 {
					return out, profileURL, "Reddit stopped showing more without an account (HTTP " + itoa(code) + "); partial"
				}
				return nil, profileURL, "Reddit did not show this account's history to a request without an account (HTTP " + itoa(code) + "): not collected"
			}
			continue
		}
		for _, ch := range r.Data.Children {
			d := ch.Data
			if d.Adult { // adult-marked items are not copied
				adult++
				continue
			}
			p := models.Post{Platform: "Reddit", Handle: name, ID: d.ID, URL: "https://www.reddit.com" + d.Permalink,
				Time: utc(time.Unix(int64(d.Created), 0)), Kind: "post", Likes: d.Score, Replies: d.Comments}
			text := strings.TrimSpace(d.Title + "\n" + d.SelfText)
			if kind == "comments" {
				p.Kind, text = "comment", d.Body
			}
			p.Text = clip(strings.TrimSpace(text), maxText)
			p.Links = links(p.Text)
			if d.Subreddit != "" {
				p.Tags = []string{"r/" + strings.ToLower(d.Subreddit)}
			}
			if kind == "submitted" && isPicture(d.URL) {
				p.Media = []string{d.URL}
			}
			out = append(out, p)
			if len(out) >= maxPosts {
				break
			}
		}
	}
	note := ""
	if adult > 0 {
		note = itoa(adult) + " adult-marked item(s) were not copied"
	}
	return out, profileURL, note
}

func isPicture(u string) bool {
	l := strings.ToLower(u)
	if i := strings.IndexAny(l, "?#"); i >= 0 {
		l = l[:i]
	}
	return strings.HasPrefix(l, "https://i.redd.it/") || strings.HasSuffix(l, ".jpg") || strings.HasSuffix(l, ".jpeg") || strings.HasSuffix(l, ".png") || strings.HasSuffix(l, ".webp")
}
