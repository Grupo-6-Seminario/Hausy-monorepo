package main

import "testing"

func TestLiveRejectsRemoteURLsDisguisedAsLoopback(t *testing.T) {
	for _, s := range []string{"http://127.0.0.1:8000@remote.example", "http://localhost:8000@remote.example", "http://remote.example:8000", "https://localhost.evil.example:8000"} {
		if loopbackEndpoint(s) == nil {
			t.Fatalf("remote URL accepted: %s", s)
		}
	}
	for _, s := range []string{"http://127.0.0.1:8000", "http://localhost:8000", "http://[::1]:8000"} {
		if err := loopbackEndpoint(s); err != nil {
			t.Fatalf("loopback rejected: %s: %v", s, err)
		}
	}
}
