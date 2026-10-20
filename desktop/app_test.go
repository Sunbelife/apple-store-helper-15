package main

import (
	"apple-store-helper/model"
	"apple-store-helper/services"
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testTarget(store string) Target {
	return Target{ID: targetID("cn", store, "MK2P4CH/A"), Area: model.Areas[0], Store: model.Store{StoreNumber: store, District: "510623"}, Product: model.Product{Code: "MK2P4CH/A", Type: "iphoneduo"}, Status: services.StatusWait}
}
func awaitState(t *testing.T, a *Desktop, predicate func(State) bool) State {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		s := a.GetState()
		if predicate(s) {
			return s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state was not reached: %+v", a.GetState())
	return State{}
}
func TestPostalResponseReusesNearbyMonitoredStores(t *testing.T) {
	a := NewDesktop()
	a.ctx = context.Background()
	a.state.Items = []Target{testTarget("R484"), testTarget("R639")}
	var requests atomic.Int32
	a.query = func(context.Context, model.Area, model.Store, []model.Product) (map[string]bool, error) {
		requests.Add(1)
		return map[string]bool{"R484.MK2P4CH/A": false, "R639.MK2P4CH/A": false}, nil
	}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Pause()
	s := awaitState(t, a, func(s State) bool { return s.NextAt > 0 })
	if requests.Load() != 1 {
		t.Fatalf("sent %d requests for one complete nearby response", requests.Load())
	}
	for _, i := range s.Items {
		if i.Status != services.StatusOutStock || i.CheckedAt == "" {
			t.Fatalf("explicit result missing: %+v", i)
		}
	}
}

func TestNextPollDelayStaysWithinRandomRange(t *testing.T) {
	for range 200 {
		delay := nextPollDelay()
		if delay < minPollDelay || delay > maxPollDelay {
			t.Fatalf("poll delay %s outside [%s, %s]", delay, minPollDelay, maxPollDelay)
		}
	}
	if got := pollDelayFor(0); got != 20*time.Second {
		t.Fatalf("lower bound = %s, want 20s", got)
	}
	if got := pollDelayFor(40); got != 60*time.Second {
		t.Fatalf("upper bound = %s, want 60s", got)
	}
}
func TestPauseCancelsInFlightRequest(t *testing.T) {
	a := NewDesktop()
	a.ctx = context.Background()
	a.state.Items = []Target{testTarget("R484")}
	entered := make(chan struct{})
	cancelled := make(chan struct{})
	a.query = func(ctx context.Context, _ model.Area, _ model.Store, _ []model.Product) (map[string]bool, error) {
		close(entered)
		<-ctx.Done()
		close(cancelled)
		return nil, ctx.Err()
	}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Pause()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("request never started")
	}
	a.Pause()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("pause did not cancel request")
	}
	if s := a.GetState(); s.Running || s.Items[0].Status == services.StatusQuerying {
		t.Fatalf("stale running state: %+v", s)
	}
}
func TestOneStoreErrorDoesNotOverwriteSuccessfulStore(t *testing.T) {
	a := NewDesktop()
	a.ctx = context.Background()
	a.state.Items = []Target{testTarget("R484"), testTarget("R639")}
	a.query = func(_ context.Context, _ model.Area, s model.Store, _ []model.Product) (map[string]bool, error) {
		if s.StoreNumber == "R484" {
			return map[string]bool{"R484.MK2P4CH/A": false}, nil
		}
		return nil, errors.New("test query failure")
	}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Pause()
	s := awaitState(t, a, func(s State) bool { return s.Items[1].Status == services.StatusError })
	if s.Items[0].Status != services.StatusOutStock {
		t.Fatalf("successful result overwritten: %+v", s.Items)
	}
}

func TestLimitedStoreDoesNotStopOtherStores(t *testing.T) {
	a := NewDesktop()
	a.ctx = context.Background()
	a.state.Items = []Target{testTarget("R484"), testTarget("R639")}
	limitedErr := errors.New("test risk control")
	a.queryErrorStatus = func(err error) string {
		if errors.Is(err, limitedErr) {
			return services.StatusLimited
		}
		return services.StatusError
	}
	requested := make(chan string, 2)
	a.query = func(_ context.Context, _ model.Area, s model.Store, _ []model.Product) (map[string]bool, error) {
		requested <- s.StoreNumber
		if s.StoreNumber == "R484" {
			return nil, limitedErr
		}
		return map[string]bool{"R639.MK2P4CH/A": false}, nil
	}
	if err := a.Start(); err != nil {
		t.Fatal(err)
	}
	defer a.Pause()
	s := awaitState(t, a, func(s State) bool {
		return s.Items[0].Status == services.StatusLimited &&
			s.Items[1].Status == services.StatusOutStock &&
			s.Phase == "部分门店冷却中，其他门店继续监控"
	})
	if s.Phase != "部分门店冷却中，其他门店继续监控" {
		t.Fatalf("unexpected phase after partial risk control: %q", s.Phase)
	}
	if len(requested) != 2 {
		t.Fatalf("sent %d requests, want both stores queried", len(requested))
	}
	if first, second := <-requested, <-requested; first != "R484" || second != "R639" {
		t.Fatalf("query order = %s, %s; want limited store followed by healthy store", first, second)
	}
	if detail := s.Items[0].Detail; !strings.Contains(detail, "仅该门店冷却") {
		t.Fatalf("limited store detail does not explain isolation: %q", detail)
	}
}

func TestScheduledPickupDoesNotTriggerStockAlert(t *testing.T) {
	a := NewDesktop()
	a.ctx = context.Background()
	a.state.Items = []Target{testTarget("R428")}
	a.query = func(context.Context, model.Area, model.Store, []model.Product) (map[string]bool, error) {
		return map[string]bool{}, &services.PickupSchedule{Quotes: map[string]string{"R428.MK2P4CH/A": "Nov 13 at Apple ifc mall"}}
	}
	alerts := make(chan struct{}, 1)
	a.onStock = func(context.Context, uint64, Target) { alerts <- struct{}{} }
	if e := a.Start(); e != nil {
		t.Fatal(e)
	}
	defer a.Pause()
	s := awaitState(t, a, func(s State) bool { return s.Items[0].Status == "可预约取货" })
	if s.Items[0].Detail != "Nov 13 at Apple ifc mall" || !s.Running {
		t.Fatalf("wrong scheduled state: %+v", s)
	}
	select {
	case <-alerts:
		t.Fatal("scheduled pickup triggered stock alert")
	default:
	}
}
