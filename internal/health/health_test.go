// Copyright (C) 2026 Seif, SUDOSOC
// SPDX-License-Identifier: AGPL-3.0-only

package health

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sudosoc/argus/internal/httpx"
)

func TestLiveHealth(t *testing.T) {
	if os.Getenv("ARGUS_LIVE") == "" {
		t.Skip("live")
	}
	c, _ := httpx.New(httpx.Options{Timeout: 20 * time.Second})
	for _, r := range Check(context.Background(), c) {
		t.Logf("%-18s ok=%v %d %dms %s", r.Name, r.OK, r.Status, r.MS, r.Error)
	}
}
