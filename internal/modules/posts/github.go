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

var githubAPI = "https://api.github.com"

type ghEvent struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Created time.Time `json:"created_at"`
	Repo    struct {
		Name string `json:"name"`
	} `json:"repo"`
	Payload struct {
		Action  string `json:"action"`
		RefType string `json:"ref_type"`
		Commits []struct {
			SHA     string `json:"sha"`
			Message string `json:"message"`
		} `json:"commits"`
		Issue struct {
			Title string `json:"title"`
			Body  string `json:"body"`
			URL   string `json:"html_url"`
		} `json:"issue"`
		PullRequest struct {
			Title string `json:"title"`
			Body  string `json:"body"`
			URL   string `json:"html_url"`
		} `json:"pull_request"`
		Comment struct {
			Body string `json:"body"`
			URL  string `json:"html_url"`
		} `json:"comment"`
		Forkee struct {
			FullName string `json:"full_name"`
			URL      string `json:"html_url"`
		} `json:"forkee"`
	} `json:"payload"`
}

func github(ctx context.Context, c *httpx.Client, handle string) ([]models.Post, string, string) {
	name := strings.TrimPrefix(handle, "@")
	profileURL := "https://github.com/" + name
	sp := &spacer{}
	sp.wait(ctx)
	var events []ghEvent
	code, ok := util.GetJSONStatus(ctx, c, githubAPI+"/users/"+url.PathEscape(name)+"/events/public?per_page=100", &events)
	switch {
	case code == 404:
		return nil, "", "no GitHub account with this name"
	case code == 403 || code == 429:
		return nil, profileURL, "GitHub refused the request (HTTP " + itoa(code) + "): requests without a key are limited per hour; try again later"
	}
	var out []models.Post
	if ok {
		for _, e := range events {
			out = append(out, ghEventPosts(e, name)...)
		}
	}
	sp.wait(ctx)
	var repos []struct {
		Name        string    `json:"name"`
		FullName    string    `json:"full_name"`
		Description string    `json:"description"`
		URL         string    `json:"html_url"`
		Language    string    `json:"language"`
		Topics      []string  `json:"topics"`
		Fork        bool      `json:"fork"`
		Stars       int       `json:"stargazers_count"`
		Forks       int       `json:"forks_count"`
		Created     time.Time `json:"created_at"`
		Pushed      time.Time `json:"pushed_at"`
	}
	if util.GetJSON(ctx, c, githubAPI+"/users/"+url.PathEscape(name)+"/repos?sort=pushed&per_page=40", &repos) {
		for _, r := range repos {
			t := r.Pushed
			if t.IsZero() {
				t = r.Created
			}
			text := r.Name
			if r.Description != "" {
				text += " — " + r.Description
			}
			p := models.Post{Platform: "GitHub", Handle: name, ID: r.FullName, URL: r.URL, Time: utc(t), Kind: "repo", Text: clip(text, maxText), Likes: r.Stars, Reposts: r.Forks}
			if r.Fork {
				p.Text = "(fork) " + p.Text
			}
			p.Tags = append(p.Tags, r.Topics...)
			if r.Language != "" {
				p.Tags = append(p.Tags, strings.ToLower(r.Language))
			}
			p.Tags = uniq(p.Tags)
			out = append(out, p)
		}
	}
	if len(out) > maxPosts {
		out = out[:maxPosts]
	}
	return out, profileURL, ""
}

func ghEventPosts(e ghEvent, handle string) []models.Post {
	base := models.Post{Platform: "GitHub", Handle: handle, Time: utc(e.Created)}
	mk := func(kind, id, url, text string) models.Post {
		p := base
		if kind == "commit" || kind == "comment" || kind == "issue" || kind == "pull" {
			text = "[" + e.Repo.Name + "] " + strings.TrimSpace(text)
		}
		p.Kind, p.ID, p.URL, p.Text = kind, id, url, clip(strings.TrimSpace(text), maxText)
		p.Links = links(p.Text)
		return p
	}
	repoURL := "https://github.com/" + e.Repo.Name
	var out []models.Post
	switch e.Type {
	case "PushEvent":
		for _, cm := range e.Payload.Commits {
			out = append(out, mk("commit", cm.SHA, repoURL+"/commit/"+cm.SHA, cm.Message))
		}
	case "IssuesEvent":
		out = append(out, mk("issue", e.ID, e.Payload.Issue.URL, e.Payload.Action+": "+e.Payload.Issue.Title+"\n"+e.Payload.Issue.Body))
	case "PullRequestEvent":
		out = append(out, mk("pull", e.ID, e.Payload.PullRequest.URL, e.Payload.Action+": "+e.Payload.PullRequest.Title+"\n"+e.Payload.PullRequest.Body))
	case "IssueCommentEvent", "PullRequestReviewCommentEvent", "CommitCommentEvent":
		out = append(out, mk("comment", e.ID, e.Payload.Comment.URL, e.Payload.Comment.Body))
	case "CreateEvent":
		if e.Payload.RefType == "repository" {
			out = append(out, mk("repo", e.ID, repoURL, "created the repository "+e.Repo.Name))
		}
	case "ForkEvent":
		out = append(out, mk("repo", e.ID, e.Payload.Forkee.URL, "forked "+e.Repo.Name))
	case "WatchEvent":
		out = append(out, mk("star", e.ID, repoURL, "starred "+e.Repo.Name))
	}
	return out
}
