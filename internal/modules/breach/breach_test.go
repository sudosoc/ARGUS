// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package breach

import (
	"github.com/sudosoc/argus/internal/util"
	"reflect"
	"testing"
)

func TestClip(t *testing.T) {
	in := []string{"a", "b", "c"}
	if got := util.ClipList(in, 2); !reflect.DeepEqual(got, in) { // nothing below the floor is dropped
		t.Errorf("clip: %v", got)
	}
	if got := util.ClipList(in, 5); !reflect.DeepEqual(got, in) {
		t.Errorf("clip longer: %v", got)
	}
	if got := util.ClipList[string](nil, 3); len(got) != 0 {
		t.Errorf("clip nil: %v", got)
	}
}

func TestSorted(t *testing.T) {
	got := sorted(map[string]bool{"b": true, "a": true, "c": true})
	if !reflect.DeepEqual(got, []string{"a", "b", "c"}) {
		t.Errorf("sorted: %v", got)
	}
	if got := sorted(nil); len(got) != 0 {
		t.Errorf("sorted nil: %v", got)
	}
}
