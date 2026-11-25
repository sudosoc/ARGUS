package iddecode

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/models"
	"github.com/sudosoc/argus/internal/scanner"
)

func TestSnowflakes(t *testing.T) {
	at := time.Date(2023, 5, 17, 10, 30, 0, 0, time.UTC)
	tw := uint64(at.UnixMilli()-twitterEpoch) << 22
	got := Decode("https://x.com/someone/status/" + itoa(tw) + "?s=20")
	if len(got) != 1 || !got[0].At.Equal(at) || got[0].Platform != "X/Twitter" {
		t.Fatalf("twitter: %+v", got)
	}
	dc := uint64(at.UnixMilli()-discordEpoch) << 22
	got = Decode("https://discord.com/users/" + itoa(dc))
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("discord: %+v", got)
	}
	tt := uint64(at.Unix()) << 32
	got = Decode("https://www.tiktok.com/@name/video/" + itoa(tt))
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("tiktok: %+v", got)
	}
	li := uint64(at.UnixMilli()) << 22
	got = Decode("https://www.linkedin.com/posts/jane-doe_hello-activity-" + itoa(li) + "-abcd")
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("linkedin: %+v", got)
	}
	ma := uint64(at.UnixMilli()) << 16
	got = Decode("https://mastodon.social/@gargron/" + itoa(ma))
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("mastodon: %+v", got)
	}
}

func TestInstagramShortcode(t *testing.T) {
	at := time.Date(2022, 1, 2, 3, 4, 5, 0, time.UTC)
	id := uint64(at.UnixMilli()-instagramEpoch) << 23
	code := ""
	for v := id; v > 0; v /= 64 {
		code = string(instaAlphabet[v%64]) + code
	}
	got := Decode("https://www.instagram.com/p/" + code + "/")
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("instagram: code=%s %+v", code, got)
	}
}

func TestBlueskyTID(t *testing.T) {
	at := time.Date(2024, 3, 4, 5, 6, 7, 123000000, time.UTC)
	v := uint64(at.UnixMicro()) << 10
	tid := ""
	for i := 0; i < 13; i++ {
		tid = string(tidAlphabet[v&31]) + tid
		v >>= 5
	}
	got := Decode("https://bsky.app/profile/alice.bsky.social/post/" + tid)
	if len(got) != 1 || !got[0].At.Equal(at) {
		t.Fatalf("bluesky: tid=%s %+v", tid, got)
	}
}

func TestIgnoresUnrelatedAndImplausible(t *testing.T) {
	for _, u := range []string{"https://example.com/a/b", "https://x.com/someone", "https://x.com/a/status/12345", "not a url", "https://discord.com/users/123"} {
		if got := Decode(u); len(got) != 0 {
			t.Errorf("%s decoded to %+v", u, got)
		}
	}
}

func TestScannerUsesLinksAndNotes(t *testing.T) {
	s := New()
	tg := models.Target{Notes: "see https://x.com/a/status/" + itoa(uint64(time.Now().AddDate(-1, 0, 0).UnixMilli()-twitterEpoch)<<22) + ", thanks"}
	if !s.CanRun(tg) {
		t.Fatal("notes with a link must enable the module")
	}
	var n int
	_ = s.Scan(context.Background(), tg, scanner.Deps{}, func(f models.Finding) {
		n++
		if f.Category != "idinfo" || f.Details["created (UTC)"] == "" {
			t.Errorf("bad finding %+v", f)
		}
	})
	if n != 1 {
		t.Errorf("findings = %d", n)
	}
	if s.CanRun(models.Target{Username: "x"}) {
		t.Error("a bare username has nothing to decode")
	}
}

func itoa(v uint64) string { return strconv.FormatUint(v, 10) }
