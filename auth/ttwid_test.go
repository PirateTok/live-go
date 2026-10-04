package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// flakyServer omits the ttwid cookie for the first `misses` requests.
func flakyServer(t *testing.T, misses int) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.SetCookie(w, &http.Cookie{Name: "tt_csrf_token", Value: "abc"})
		if hits > misses {
			http.SetCookie(w, &http.Cookie{Name: "ttwid", Value: "1%7Cfresh", Path: "/", HttpOnly: true})
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestFetchTTWIDMissingCookieThenCookie(t *testing.T) {
	srv, hits := flakyServer(t, 5)
	got, err := fetchTTWID(srv.Client(), srv.URL, "test-ua", FetchAttempts, 0)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if got != "1%7Cfresh" {
		t.Fatalf("ttwid = %q", got)
	}
	if *hits != 6 {
		t.Fatalf("requests = %d, want 6", *hits)
	}
}

func TestFetchTTWIDFirstHitNoRetry(t *testing.T) {
	srv, hits := flakyServer(t, 0)
	if _, err := fetchTTWID(srv.Client(), srv.URL, "test-ua", FetchAttempts, 0); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if *hits != 1 {
		t.Fatalf("requests = %d, want 1", *hits)
	}
}

func TestFetchTTWIDNeverCookieFailsAfterEight(t *testing.T) {
	srv, hits := flakyServer(t, 1<<30)
	_, err := fetchTTWID(srv.Client(), srv.URL, "test-ua", FetchAttempts, 0)
	if err == nil || !strings.Contains(err.Error(), "after 8 attempts") {
		t.Fatalf("err = %v, want 'after 8 attempts'", err)
	}
	if *hits != 8 {
		t.Fatalf("requests = %d, want 8", *hits)
	}
}

func TestFetchTTWIDTransportErrorNoRetry(t *testing.T) {
	srv, _ := flakyServer(t, 0)
	target := srv.URL
	srv.Close()
	start := time.Now()
	if _, err := fetchTTWID(&http.Client{Timeout: time.Second}, target, "test-ua", FetchAttempts, time.Second); err == nil {
		t.Fatal("expected transport error")
	}
	if time.Since(start) >= time.Second {
		t.Fatal("transport error was retried")
	}
}

func TestFetchDefaultsMatchReference(t *testing.T) {
	if FetchAttempts != 8 || RetryDelay != 750*time.Millisecond {
		t.Fatalf("attempts=%d delay=%v", FetchAttempts, RetryDelay)
	}
}
