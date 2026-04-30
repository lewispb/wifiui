package portal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("success\n"))
	}))
	defer srv.Close()

	r := checkURL(context.Background(), srv.URL)
	if r.Status != StatusOK {
		t.Errorf("Status = %v, want OK", r.Status)
	}
}

func TestCheckPortalRedirect(t *testing.T) {
	const target = "http://login.example.com/welcome"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", target)
		w.WriteHeader(http.StatusFound)
	}))
	defer srv.Close()

	r := checkURL(context.Background(), srv.URL)
	if r.Status != StatusPortal {
		t.Errorf("Status = %v, want Portal", r.Status)
	}
	if r.RedirectURL != target {
		t.Errorf("RedirectURL = %q, want %q", r.RedirectURL, target)
	}
}

func TestCheckPortalUnexpectedBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("<html>Sign in here</html>"))
	}))
	defer srv.Close()

	r := checkURL(context.Background(), srv.URL)
	if r.Status != StatusPortal {
		t.Errorf("Status = %v, want Portal", r.Status)
	}
	if r.RedirectURL == "" {
		t.Errorf("RedirectURL empty; expected the probe URL as fallback")
	}
}

func TestCheckOffline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // immediately close — connections will fail

	r := checkURL(context.Background(), srv.URL)
	if r.Status != StatusOffline {
		t.Errorf("Status = %v, want Offline", r.Status)
	}
	if r.Err == nil {
		t.Errorf("expected Err on offline result")
	}
}

func TestStatusString(t *testing.T) {
	cases := map[Status]string{
		StatusOK:      "ok",
		StatusPortal:  "portal",
		StatusOffline: "offline",
		StatusUnknown: "unknown",
	}
	for s, want := range cases {
		if got := s.String(); got != want {
			t.Errorf("Status(%d).String() = %q, want %q", s, got, want)
		}
	}
}
