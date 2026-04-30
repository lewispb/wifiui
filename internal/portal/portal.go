// Package portal detects whether the current connection is behind a captive
// portal by probing a well-known plaintext endpoint. The probe is cheap (one
// HTTP GET) and disables redirects so we can capture the portal target URL.
package portal

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"
)

type Status int

const (
	StatusUnknown Status = iota
	StatusOK             // probe matches expected body — internet reachable
	StatusPortal         // probe redirected or returned unexpected content
	StatusOffline        // probe failed (no connectivity)
)

func (s Status) String() string {
	switch s {
	case StatusOK:
		return "ok"
	case StatusPortal:
		return "portal"
	case StatusOffline:
		return "offline"
	default:
		return "unknown"
	}
}

type Result struct {
	Status      Status
	RedirectURL string
	CheckedAt   time.Time
	Err         error
}

// Probe URL and expected body. Firefox's detectportal endpoint is widely
// whitelisted by captive portals (so we observe the redirect rather than
// being silently passed through).
const (
	probeURL      = "http://detectportal.firefox.com/success.txt"
	expectedBody  = "success"
	probeTimeout  = 5 * time.Second
)

// Check probes for a captive portal. It returns quickly even on slow networks
// because it doesn't follow redirects.
func Check(ctx context.Context) Result {
	return checkURL(ctx, probeURL)
}

func checkURL(ctx context.Context, url string) Result {
	cctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()

	client := &http.Client{
		Timeout: probeTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, _ := http.NewRequestWithContext(cctx, http.MethodGet, url, nil)
	resp, err := client.Do(req)
	if err != nil {
		return Result{Status: StatusOffline, CheckedAt: time.Now(), Err: err}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return Result{
			Status:      StatusPortal,
			RedirectURL: resp.Header.Get("Location"),
			CheckedAt:   time.Now(),
		}
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if strings.TrimSpace(string(body)) == expectedBody {
		return Result{Status: StatusOK, CheckedAt: time.Now()}
	}
	return Result{Status: StatusPortal, RedirectURL: url, CheckedAt: time.Now()}
}
