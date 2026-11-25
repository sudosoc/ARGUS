// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package models

import (
	"reflect"
	"testing"
)

func TestTargetIsEmpty(t *testing.T) {
	if !(Target{}).IsEmpty() {
		t.Error("zero target should be empty")
	}
	if !(Target{Location: "Cairo"}).IsEmpty() {
		t.Error("location alone is not an identifier")
	}
	if !(Target{Notes: "  \n "}).IsEmpty() {
		t.Error("blank notes are not an identifier")
	}
	for _, tg := range []Target{{FullName: "A"}, {Username: "a"}, {Email: "a@b.c"}, {Phone: "1"}, {Domain: "a.b"},
		{IP: "1.1.1.1"}, {Company: "X"}, {Alias: "a"}, {ASN: "AS1"}, {Wallet: "w"}, {Notes: "cv"}} {
		if tg.IsEmpty() {
			t.Errorf("%+v should not be empty", tg)
		}
	}
}

func TestTargetUsernames(t *testing.T) {
	got := Target{Username: "bob", Aliases: []string{"bob", "", "bobby", "bobby", "b0b"}}.Usernames()
	if want := []string{"bob", "bobby", "b0b"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v want %v", got, want)
	}
	if got := (Target{Aliases: []string{"x"}}).Usernames(); !reflect.DeepEqual(got, []string{"x"}) {
		t.Errorf("alias only: %v", got)
	}
	if got := (Target{}).Usernames(); len(got) != 0 {
		t.Errorf("empty: %v", got)
	}
}

func TestReportHits(t *testing.T) {
	r := Report{Findings: []Finding{
		{Source: "a", Status: StatusFound}, {Source: "b", Status: StatusNotFound},
		{Source: "c", Status: StatusUncertain}, {Source: "d", Status: StatusError},
	}}
	h := r.Hits()
	if len(h) != 2 || h[0].Source != "a" || h[1].Source != "c" {
		t.Errorf("Hits() = %+v", h)
	}
}
