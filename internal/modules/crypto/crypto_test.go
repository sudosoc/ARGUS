// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package crypto

import "testing"

func TestBTC(t *testing.T) {
	if got := btc(150000000); got != "1.50000000 BTC" {
		t.Errorf("got %q", got)
	}
	if got := btc(0); got != "0.00000000 BTC" {
		t.Errorf("got %q", got)
	}
}

func TestWei(t *testing.T) {
	if got := wei("1500000000000000000"); got != "1.500000 ETH" {
		t.Errorf("got %q", got)
	}
	if got := wei("0"); got != "0.000000 ETH" {
		t.Errorf("got %q", got)
	}
	if got := wei("not-a-number"); got != "not-a-number" {
		t.Errorf("invalid input must pass through, got %q", got)
	}
}
