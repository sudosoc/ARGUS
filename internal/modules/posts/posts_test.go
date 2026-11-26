// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package posts

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
	"github.com/sudosoc/argus/internal/media"
	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/pool"
	"github.com/sudosoc/argus/internal/scanner"
)

func client(t *testing.T) *httpx.Client {
	t.Helper()
	c, err := httpx.New(httpx.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func fast(t *testing.T) {
	t.Helper()
	media.AllowPrivateHostsForTests(true)
	old := pause
	pause = 0
	t.Cleanup(func() { media.AllowPrivateHostsForTests(false); pause = old })
}

func TestMastodonCopiesPublicPostsAndRespectsNoindex(t *testing.T) {
	fast(t)
	noindex := false
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/accounts/lookup"):
			fmt.Fprintf(w, `{"id":"7","url":"https://m.example/@ann","noindex":%v}`, noindex)
		case strings.HasSuffix(r.URL.Path, "/statuses"):
			fmt.Fprint(w, `[
			 {"id":"3","created_at":"2026-03-02T10:00:00Z","url":"https://m.example/@ann/3","language":"en","favourites_count":4,"reblogs_count":1,"replies_count":2,"visibility":"public",
			  "content":"<p>Hello <a href=\"x\">#DFIR</a> folks<br>see https://example.org/a.</p>","tags":[{"name":"DFIR"}],"mentions":[{"acct":"bob@m.example"}],
			  "media_attachments":[{"type":"image","url":"https://m.example/i.png","preview_url":"https://m.example/p.png"}],"account":{"acct":"ann"}},
			 {"id":"2","created_at":"2026-03-01T10:00:00Z","in_reply_to_id":"1","visibility":"public","content":"<p>a reply</p>","account":{"acct":"ann"}},
			 {"id":"1","created_at":"2026-02-01T10:00:00Z","visibility":"private","content":"<p>followers only</p>","account":{"acct":"ann"}},
			 {"id":"9","created_at":"2026-02-02T10:00:00Z","visibility":"public","content":"","reblog":{"id":"8","content":"<p>original</p>","account":{"acct":"carol@x.example"}},"account":{"acct":"ann"}}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	oldHosts, oldBase := mastodonHosts, mastodonBase
	mastodonHosts, mastodonBase = []string{"test.example"}, func(string) string { return srv.URL }
	defer func() { mastodonHosts, mastodonBase = oldHosts, oldBase }()

	items, profile, note := mastodon(context.Background(), client(t), "ann")
	if note != "" || profile != "https://m.example/@ann" || len(items) != 3 {
		t.Fatalf("items=%d profile=%q note=%q", len(items), profile, note)
	}
	p := items[0]
	if p.Kind != "post" || p.Text != "Hello #DFIR folks\nsee https://example.org/a." || len(p.Tags) != 1 || p.Tags[0] != "dfir" || len(p.Links) != 1 || p.Links[0] != "https://example.org/a" || len(p.Media) != 1 || p.Likes != 4 {
		t.Errorf("post not copied properly: %+v", p)
	}
	if items[1].Kind != "reply" || items[2].Kind != "boost" || items[2].Author != "carol@x.example" || items[2].Text != "original" {
		t.Errorf("reply/boost: %+v %+v", items[1], items[2])
	}
	for _, it := range items {
		if strings.Contains(it.Text, "followers only") {
			t.Error("a non-public post was copied")
		}
	}

	noindex, calls = true, 0
	items, _, note = mastodon(context.Background(), client(t), "ann")
	if len(items) != 0 || !strings.Contains(note, "noindex") || calls != 1 {
		t.Errorf("noindex account must not be collected: items=%d note=%q calls=%d", len(items), note, calls)
	}
}

func TestBlueskyCopiesFeedAndHonoursLoggedOutOptOut(t *testing.T) {
	fast(t)
	optOut := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/app.bsky.actor.getProfile":
			labels := `[]`
			if optOut {
				labels = `[{"val":"!no-unauthenticated"}]`
			}
			fmt.Fprintf(w, `{"did":"did:plc:abc","handle":"ann.bsky.social","labels":%s}`, labels)
		case "/xrpc/app.bsky.feed.getAuthorFeed":
			fmt.Fprint(w, `{"feed":[
			 {"post":{"uri":"at://did:plc:abc/app.bsky.feed.post/3k1","author":{"handle":"ann.bsky.social"},"likeCount":5,
			   "record":{"text":"Writing about #osint with @bob.example.com https://example.org/x","createdAt":"2026-05-01T12:00:00Z","langs":["en"],
			    "facets":[{"features":[{"$type":"app.bsky.richtext.facet#tag","tag":"osint"}]},{"features":[{"$type":"app.bsky.richtext.facet#link","uri":"https://example.org/x"}]}]},
			   "embed":{"images":[{"fullsize":"https://cdn.example/i.jpg"}]}}},
			 {"post":{"uri":"at://did:plc:xyz/app.bsky.feed.post/3k2","author":{"handle":"dina.example"},"record":{"text":"original","createdAt":"2026-04-01T12:00:00Z"}},"reason":{"$type":"app.bsky.feed.defs#reasonRepost"}},
			 {"post":{"uri":"at://did:plc:abc/app.bsky.feed.post/3k3","author":{"handle":"ann.bsky.social"},"record":{"text":"a reply","createdAt":"2026-03-01T12:00:00Z"}},"reply":{"parent":{"author":{"handle":"bob.example.com"}}}}]}`)
		}
	}))
	defer srv.Close()
	old := blueskyAPI
	blueskyAPI = srv.URL + "/xrpc"
	defer func() { blueskyAPI = old }()

	items, profile, note := bluesky(context.Background(), client(t), "ann")
	if note != "" || profile != "https://bsky.app/profile/ann.bsky.social" || len(items) != 3 {
		t.Fatalf("items=%d profile=%q note=%q", len(items), profile, note)
	}
	p := items[0]
	if p.Kind != "post" || p.Lang != "en" || len(p.Tags) != 1 || p.Tags[0] != "osint" || len(p.Mentions) != 1 || p.Mentions[0] != "bob.example.com" || len(p.Media) != 1 || p.Likes != 5 ||
		p.URL != "https://bsky.app/profile/ann.bsky.social/post/3k1" {
		t.Errorf("post: %+v", p)
	}
	if items[1].Kind != "boost" || items[1].Author != "dina.example" || items[2].Kind != "reply" || items[2].ReplyTo != "bob.example.com" {
		t.Errorf("boost/reply: %+v %+v", items[1], items[2])
	}
	optOut = true
	items, _, note = bluesky(context.Background(), client(t), "ann")
	if len(items) != 0 || !strings.Contains(note, "asked not to be shown") {
		t.Errorf("an account that asked not to be shown to logged-out viewers must not be collected: %d %q", len(items), note)
	}
}

func TestGitHubEventsAndRepos(t *testing.T) {
	fast(t)
	limited := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limited {
			w.WriteHeader(403)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/events/public"):
			fmt.Fprint(w, `[
			 {"id":"1","type":"PushEvent","created_at":"2026-06-01T10:00:00Z","repo":{"name":"ann/tool"},"payload":{"commits":[{"sha":"abc","message":"Fix parser\n\nlong body"}]}},
			 {"id":"2","type":"IssueCommentEvent","created_at":"2026-06-02T10:00:00Z","repo":{"name":"org/x"},"payload":{"comment":{"body":"Thanks, works","html_url":"https://github.com/org/x/issues/1#c"}}},
			 {"id":"3","type":"WatchEvent","created_at":"2026-06-03T10:00:00Z","repo":{"name":"org/y"},"payload":{}},
			 {"id":"4","type":"MemberEvent","created_at":"2026-06-03T10:00:00Z","repo":{"name":"org/z"},"payload":{}}]`)
		case strings.HasSuffix(r.URL.Path, "/repos"):
			fmt.Fprint(w, `[{"name":"tool","full_name":"ann/tool","description":"A DFIR helper","html_url":"https://github.com/ann/tool","language":"Go","topics":["dfir","osint"],"stargazers_count":12,"pushed_at":"2026-06-01T10:00:00Z"}]`)
		}
	}))
	defer srv.Close()
	old := githubAPI
	githubAPI = srv.URL
	defer func() { githubAPI = old }()

	items, profile, note := github(context.Background(), client(t), "ann")
	if note != "" || profile != "https://github.com/ann" || len(items) != 4 {
		t.Fatalf("items=%d profile=%q note=%q: %+v", len(items), profile, note, items)
	}
	kinds := map[string]int{}
	for _, it := range items {
		kinds[it.Kind]++
	}
	if kinds["commit"] != 1 || kinds["comment"] != 1 || kinds["star"] != 1 || kinds["repo"] != 1 {
		t.Errorf("kinds %v", kinds)
	}
	limited = true
	items, _, note = github(context.Background(), client(t), "ann")
	if len(items) != 0 || !strings.Contains(note, "refused") {
		t.Errorf("rate limit must be reported, not hidden: %d %q", len(items), note)
	}
}

func TestRedditSkipsAdultAndReportsRefusal(t *testing.T) {
	fast(t)
	refuse := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if refuse {
			w.WriteHeader(403)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/submitted.json") {
			fmt.Fprint(w, `{"data":{"children":[
			 {"data":{"id":"a","created_utc":1780000000,"subreddit":"golang","permalink":"/r/golang/comments/a/x/","title":"My tool","selftext":"details","score":10,"num_comments":2,"url":"https://i.redd.it/p.png"}},
			 {"data":{"id":"b","created_utc":1780000100,"subreddit":"nsfwthing","permalink":"/r/n/comments/b/x/","title":"adult","over_18":true}}]}}`)
			return
		}
		fmt.Fprint(w, `{"data":{"children":[{"data":{"id":"c","created_utc":1780000200,"subreddit":"netsec","permalink":"/r/netsec/comments/c/x/d/","body":"good point https://example.org/z"}}]}}`)
	}))
	defer srv.Close()
	old := redditBase
	redditBase = srv.URL
	defer func() { redditBase = old }()

	items, _, note := reddit(context.Background(), client(t), "ann")
	if len(items) != 2 || !strings.Contains(note, "adult") {
		t.Fatalf("items=%d note=%q", len(items), note)
	}
	if items[0].Tags[0] != "r/golang" || len(items[0].Media) != 1 || items[1].Kind != "comment" || len(items[1].Links) != 1 {
		t.Errorf("%+v", items)
	}
	refuse = true
	items, _, note = reddit(context.Background(), client(t), "ann")
	if len(items) != 0 || !strings.Contains(note, "did not show") {
		t.Errorf("a refusal must be reported: %d %q", len(items), note)
	}
}

func TestOnlyTypedUsernamesAreUsedAndOptIn(t *testing.T) {
	s := New()[0]
	if !s.OptIn() {
		t.Error("posts collectors must be opt-in")
	}
	// derived aliases are guesses: never used
	tg := models.Target{Aliases: []string{"guess1", "guess2"}}
	if s.CanRun(tg) {
		t.Error("must not run on derived aliases")
	}
	tg = models.Target{Username: "ann", Alias: "ann", Aliases: []string{"guess"}}
	if got := handles(tg); len(got) != 1 || got[0] != "ann" {
		t.Errorf("handles %v", got)
	}
	// name@server names an instance, so only Mastodon can use it
	tg = models.Target{Username: "ann@infosec.exchange"}
	if !New()[0].CanRun(tg) || New()[1].CanRun(tg) {
		t.Error("user@server is for Mastodon only")
	}
}

func TestScanEmitsOneFindingPerAccountWithItems(t *testing.T) {
	fast(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/events/public"):
			fmt.Fprint(w, `[{"id":"3","type":"WatchEvent","created_at":"2026-06-03T10:00:00Z","repo":{"name":"org/y"},"payload":{}}]`)
		default:
			fmt.Fprint(w, `[]`)
		}
	}))
	defer srv.Close()
	old := githubAPI
	githubAPI = srv.URL
	defer func() { githubAPI = old }()
	c := client(t)
	wp := pool.New(4, 8)
	defer wp.Close()
	var got []models.Finding
	err := New()[2].Scan(context.Background(), models.Target{Username: "ann"}, scanner.Deps{HTTP: c, Pool: wp}, func(f models.Finding) { got = append(got, f) })
	if err != nil || len(got) != 1 {
		t.Fatalf("err=%v findings=%d", err, len(got))
	}
	f := got[0]
	if f.Status != models.StatusFound || len(f.Posts) != 1 || f.PostSource == nil || f.PostSource.Count != 1 || f.PostSource.Newest != "2026-06-03T10:00:00Z" || f.Category != "posts" {
		t.Errorf("%+v", f)
	}
	_ = time.Now
}
