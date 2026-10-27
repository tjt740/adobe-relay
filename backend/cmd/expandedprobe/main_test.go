package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestGateHonorsRequestedConcurrencyAndBudget(t *testing.T) {
	for _, n := range []int{1, 3, 8} {
		g := &gate{max: 12, deadline: time.Now().Add(time.Second)}
		var wg sync.WaitGroup
		for w := 0; w < n; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					_, ok := g.take(context.Background())
					if !ok {
						return
					}
					time.Sleep(5 * time.Millisecond)
					g.finish(200)
				}
			}()
		}
		wg.Wait()
		if g.next != 12 || g.peak != n || g.active != 0 {
			t.Fatalf("concurrency %d: %+v", n, g)
		}
	}
}
func TestGateStopsNewJobsOnAuthenticationFailure(t *testing.T) {
	g := &gate{max: 12, deadline: time.Now().Add(time.Second)}
	for i := 0; i < 3; i++ {
		g.take(context.Background())
	}
	g.finish(401)
	if _, ok := g.take(context.Background()); ok {
		t.Fatal("scheduled work after 401")
	}
	g.finish(200)
	g.finish(200)
	if g.next != 3 || g.active != 0 {
		t.Fatal("accepted work was not accounted for")
	}
}
func Test429AppliesSharedCooldownAnd408DoesNot(t *testing.T) {
	g := &gate{max: 12, deadline: time.Now().Add(time.Minute)}
	g.take(context.Background())
	g.finish(408)
	if !g.cooldown.IsZero() {
		t.Fatal("changed native 408 policy")
	}
	g.take(context.Background())
	g.finish(429)
	if time.Until(g.cooldown) < 14*time.Second {
		t.Fatal("missing native 15-second cooldown")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, ok := g.take(ctx); ok {
		t.Fatal("ignored shared cooldown")
	}
}

func TestLowFrequencyGateRespectsStartSpacing(t *testing.T) {
	g := &gate{max: 2, deadline: time.Now().Add(time.Second), interval: 50 * time.Millisecond}
	_, ok := g.take(context.Background())
	if !ok {
		t.Fatal("first job blocked")
	}
	g.finish(200)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	if _, ok := g.take(ctx); ok {
		t.Fatal("second job violated minimum spacing")
	}
	if g.next != 1 {
		t.Fatal("cancelled wait consumed job budget")
	}
}
