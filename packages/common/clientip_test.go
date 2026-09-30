package common

import (
	"net/http/httptest"
	"testing"
)

func TestClientIPHonoursOnlyTrustedProxyHops(t *testing.T) {
	get := func(hops, xff, remote string) string {
		t.Setenv("TRUSTED_PROXY_HOPS", hops)
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = remote
		if xff != "" {
			r.Header.Set("X-Forwarded-For", xff)
		}
		return ClientIP(r)
	}
	// Gateway exposed directly: the client's own header is ignored, only what our proxy appended counts.
	if got := get("0", "6.6.6.6, 203.0.113.9", "10.0.0.2:1"); got != "203.0.113.9" {
		t.Errorf("hops=0: %s", got)
	}
	// One TLS proxy in front: [client, proxy]; the client is the entry before the proxy's.
	if got := get("1", "203.0.113.9, 172.18.0.5", "10.0.0.2:1"); got != "203.0.113.9" {
		t.Errorf("hops=1: %s", got)
	}
	// A forged prefix cannot displace the real client.
	if got := get("1", "6.6.6.6, 203.0.113.9, 172.18.0.5", "10.0.0.2:1"); got != "203.0.113.9" {
		t.Errorf("hops=1 forged: %s", got)
	}
	if got := get("1", "", "198.51.100.7:4444"); got != "198.51.100.7" {
		t.Errorf("no header: %s", got)
	}
	if got := get("3", "203.0.113.9", "10.0.0.2:1"); got != "203.0.113.9" {
		t.Errorf("hops beyond the chain: %s", got)
	}
}
