package devtrace

import (
	"github.com/sudosoc/argus/internal/util"
	"reflect"
	"testing"
)

func TestIsNoReply(t *testing.T) {
	for _, e := range []string{"noreply@github.com", "123+bot@users.NOREPLY.github.com", "root@localhost", "a@host.local", "a@b.(none)", "x@example.com"} {
		if !isNoReply(e) {
			t.Errorf("%q should be a no-reply address", e)
		}
	}
	for _, e := range []string{"jane@acme.org", "dev@gmail.com"} {
		if isNoReply(e) {
			t.Errorf("%q is a real address", e)
		}
	}
}

func TestHas(t *testing.T) {
	if !has([]string{"Go", "Rust"}, "go") || has([]string{"Go"}, "c") || has(nil, "x") {
		t.Error("has")
	}
}

func TestTop(t *testing.T) {
	got := top(map[string]int{"go": 5, "c": 5, "rust": 1, "zig": 2}, 3)
	if got != "c (5), go (5), zig (2)" {
		t.Errorf("got %q", got)
	}
	if top(nil, 3) != "" {
		t.Error("nil map")
	}
}

func TestClip(t *testing.T) {
	if got := util.ClipList([]string{"a", "b", "c"}, 2); !reflect.DeepEqual(got, []string{"a", "b", "c"}) { // nothing below the floor is dropped
		t.Errorf("%v", got)
	}
}
