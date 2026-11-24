package pool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoolRunsAllTasks(t *testing.T) {
	p := New(4, 2)
	var n atomic.Int64
	for i := 0; i < 200; i++ {
		if err := p.Submit(context.Background(), func() { n.Add(1) }); err != nil {
			t.Fatal(err)
		}
	}
	p.Close()
	if n.Load() != 200 {
		t.Fatalf("ran %d tasks", n.Load())
	}
}

func TestPoolBoundsConcurrency(t *testing.T) {
	p := New(3, 0)
	var cur, max atomic.Int64
	for i := 0; i < 30; i++ {
		_ = p.Submit(context.Background(), func() {
			c := cur.Add(1)
			for {
				m := max.Load()
				if c <= m || max.CompareAndSwap(m, c) {
					break
				}
			}
			time.Sleep(2 * time.Millisecond)
			cur.Add(-1)
		})
	}
	p.Close()
	if max.Load() > 3 {
		t.Fatalf("concurrency %d exceeded 3 workers", max.Load())
	}
}

func TestPoolSurvivesPanic(t *testing.T) {
	p := New(1, 1)
	var ok atomic.Bool
	_ = p.Submit(context.Background(), func() { panic("boom") })
	_ = p.Submit(context.Background(), func() { ok.Store(true) })
	p.Close()
	if !ok.Load() {
		t.Fatal("worker died after panic")
	}
}

func TestSubmitHonoursContext(t *testing.T) {
	p := New(1, 0)
	block := make(chan struct{})
	_ = p.Submit(context.Background(), func() { <-block })
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	ran := false
	if err := p.Submit(ctx, func() { ran = true }); err == nil {
		t.Fatal("expected context error while the pool is saturated")
	}
	cctx, c2 := context.WithCancel(context.Background())
	c2()
	if err := p.Submit(cctx, func() { ran = true }); err == nil {
		t.Fatal("expected error for cancelled context")
	}
	close(block)
	p.Close()
	if ran {
		t.Fatal("rejected task must not run")
	}
}

func TestCloseIdempotentAndClampedArgs(t *testing.T) {
	p := New(0, -5)
	p.Close()
	p.Close()
}
