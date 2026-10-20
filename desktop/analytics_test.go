package main

import "testing"

func TestUsageRequiresConsentAndNeverGeneratesAnIdentity(t *testing.T) {
	a := NewDesktop()
	if e := a.TrackUsage("app_open"); e != nil {
		t.Fatal(e)
	}
	if a.usageIdentity != "" {
		t.Fatal("identity created before consent")
	}
	if e := a.SetUsageIdentity("hardware-serial"); e == nil {
		t.Fatal("invalid identifier accepted")
	}
	if e := a.SetUsageIdentity("11111111-1111-4111-8111-111111111111"); e != nil {
		t.Fatal(e)
	}
	if e := a.TrackUsage("secret-content"); e == nil {
		t.Fatal("unknown event accepted")
	}
}
