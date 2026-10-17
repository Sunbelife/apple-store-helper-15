package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"apple-store-helper/model"
	"apple-store-helper/services"
)

// This opt-in test runs the real production monitoring loop and HTTP client.
// No fake stock, cached inventory or shortened polling intervals are used.
func TestLiveDesktopSoak(t *testing.T) {
	if os.Getenv("QNM_LIVE_SOAK") != "1" {
		t.Skip("opt-in eleven-minute live inventory test")
	}
	path := os.Getenv("QNM_SOAK_REPORT")
	if path == "" {
		t.Fatal("QNM_SOAK_REPORT is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	out := json.NewEncoder(f)
	a := NewDesktop()
	a.ctx = context.Background()
	catalog, err := a.GetCatalog("cn")
	if err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{"R484": true, "R577": true, "R639": true, "R761": true, "R793": true}
	for _, s := range catalog.Stores {
		if wanted[s.StoreNumber] {
			a.state.Items = append(a.state.Items, Target{ID: targetID("cn", s.StoreNumber, "MK2P4CH/A"), Area: model.Areas[0], Store: s, Product: model.Product{Code: "MK2P4CH/A", Type: "iphoneduo", Title: "iPhone Duo 512GB 星光白色"}, Status: services.StatusWait})
		}
	}
	if len(a.state.Items) != 5 {
		t.Fatalf("expected all five actual targets, got %d", len(a.state.Items))
	}
	a.onStock = func(context.Context, uint64, Target) { a.Pause() }
	started := time.Now()
	duration := 11 * time.Minute
	out.Encode(map[string]any{"event": "start", "at": started.Format(time.RFC3339), "duration_seconds": duration.Seconds(), "targets": 5})
	if err = a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Pause()
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()
	var previous int64
	rounds := 0
	var lastSuccess time.Time
	for {
		select {
		case <-deadline.C:
			elapsed := time.Since(started)
			if rounds < 20 {
				t.Fatalf("only %d successful rounds in %s", rounds, elapsed)
			}
			if time.Since(lastSuccess) > 60*time.Second {
				t.Fatal("no fresh result at end")
			}
			out.Encode(map[string]any{"event": "passed", "at": time.Now().Format(time.RFC3339), "elapsed_seconds": elapsed.Seconds(), "successful_rounds": rounds, "targets_per_round": 5})
			t.Logf("PASS: real engine ran %s, %d complete fresh rounds for all five stores", elapsed, rounds)
			return
		case <-tick.C:
			state := a.GetState()
			if !state.Running {
				t.Fatal("monitor stopped during soak (possibly real stock appeared)")
			}
			if state.NextAt == 0 || state.NextAt == previous {
				continue
			}
			previous = state.NextAt
			now := time.Now()
			record := map[string]any{"event": "round", "at": now.Format(time.RFC3339), "elapsed_seconds": time.Since(started).Seconds(), "state": state}
			out.Encode(record)
			f.Sync()
			for _, target := range state.Items {
				if target.Status != services.StatusOutStock && target.Status != services.StatusInStock {
					t.Fatalf("unstable round: %s = %s (%s)", target.Store.StoreNumber, target.Status, target.Detail)
				}
				checked, err := time.Parse(time.RFC3339, target.CheckedAt)
				if err != nil || now.Sub(checked) > 45*time.Second {
					t.Fatalf("stale result: %+v", target)
				}
			}
			if !lastSuccess.IsZero() && now.Sub(lastSuccess) > 65*time.Second {
				t.Fatal("excessive gap between complete rounds")
			}
			lastSuccess = now
			rounds++
			t.Logf("round %d: five fresh explicit inventory results", rounds)
		}
	}
}
