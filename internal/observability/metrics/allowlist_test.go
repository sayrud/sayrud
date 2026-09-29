// Copyright 2026 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRestrictAccess(t *testing.T) {
	for _, tc := range []struct {
		name      string
		whitelist []string
		remote    string
		allowed   bool
	}{
		{name: "IPv4 exact", whitelist: []string{"192.0.2.7"}, remote: "192.0.2.7:12345", allowed: true},
		{name: "IPv4 other address", whitelist: []string{"192.0.2.7"}, remote: "192.0.2.8:12345"},
		{name: "IPv4 subnet first", whitelist: []string{"192.0.2.0/24"}, remote: "192.0.2.0:12345", allowed: true},
		{name: "IPv4 subnet last", whitelist: []string{"192.0.2.0/24"}, remote: "192.0.2.255:12345", allowed: true},
		{name: "IPv4 below subnet", whitelist: []string{"192.0.2.0/24"}, remote: "192.0.1.255:12345"},
		{name: "IPv4 above subnet", whitelist: []string{"192.0.2.0/24"}, remote: "192.0.3.0:12345"},
		{name: "CIDR host bits masked", whitelist: []string{"192.0.2.129/25"}, remote: "192.0.2.128:12345", allowed: true},
		{name: "CIDR host bits boundary", whitelist: []string{"192.0.2.129/25"}, remote: "192.0.2.127:12345"},
		{name: "IPv6 exact equivalent spelling", whitelist: []string{"2001:db8::1"}, remote: "[2001:0db8:0000:0000:0000:0000:0000:0001]:12345", allowed: true},
		{name: "IPv6 other address", whitelist: []string{"2001:db8::1"}, remote: "[2001:db8::2]:12345"},
		{name: "IPv6 subnet", whitelist: []string{"2001:db8:abcd::/48"}, remote: "[2001:db8:abcd:ffff:ffff:ffff:ffff:ffff]:12345", allowed: true},
		{name: "IPv6 outside subnet", whitelist: []string{"2001:db8:abcd::/48"}, remote: "[2001:db8:abce::]:12345"},
		{name: "IPv4 mapped remote exact", whitelist: []string{"192.0.2.7"}, remote: "[::ffff:192.0.2.7]:12345", allowed: true},
		{name: "IPv4 mapped remote subnet", whitelist: []string{"192.0.2.0/24"}, remote: "[::ffff:c000:0207]:12345", allowed: true},
		{name: "IPv4 mapped whitelist", whitelist: []string{"::ffff:192.0.2.7"}, remote: "192.0.2.7:12345", allowed: true},
		{name: "IPv4 mapped CIDR", whitelist: []string{"::ffff:192.0.2.0/120"}, remote: "192.0.2.7:12345", allowed: true},
		{name: "IPv4 mapped CIDR outside", whitelist: []string{"::ffff:192.0.2.0/120"}, remote: "192.0.3.7:12345"},
		{name: "whitespace", whitelist: []string{" \t192.0.2.7\n"}, remote: "192.0.2.7:12345", allowed: true},
		{name: "multiple entries", whitelist: []string{"192.0.2.7", "2001:db8::/32"}, remote: "[2001:db8::1]:12345", allowed: true},
		{name: "nil whitelist", remote: "127.0.0.1:12345"},
		{name: "empty whitelist", whitelist: []string{}, remote: "[::1]:12345"},
		{name: "bare IP", whitelist: []string{"127.0.0.1"}, remote: "127.0.0.1"},
		{name: "hostname", whitelist: []string{"127.0.0.1"}, remote: "localhost:12345"},
		{name: "IPv6 zone", whitelist: []string{"fe80::/10"}, remote: "[fe80::1%en0]:12345"},
		{name: "empty remote", whitelist: []string{"0.0.0.0/0", "::/0"}},
		{name: "empty port", whitelist: []string{"127.0.0.1"}, remote: "127.0.0.1:"},
		{name: "invalid port", whitelist: []string{"127.0.0.1"}, remote: "127.0.0.1:http"},
		{name: "port out of range", whitelist: []string{"127.0.0.1"}, remote: "127.0.0.1:65536"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			handler, err := restrictAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}), tc.whitelist)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)
			r.RemoteAddr = tc.remote
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			wantStatus := http.StatusForbidden
			if tc.allowed {
				wantStatus = http.StatusNoContent
			}
			if w.Code != wantStatus || called != tc.allowed {
				t.Fatalf("status = %d, next called = %t; want status %d, next called %t", w.Code, called, wantStatus, tc.allowed)
			}
		})
	}
}

func TestRestrictAccessInvalidWhitelist(t *testing.T) {
	for _, entry := range []string{
		"", " \t", "localhost", "*", "192.0.2.1:12345", "192.0.2.256",
		"192.0.2.0/33", "192.0.2.0/-1", "2001:db8::/129", "192.0.2.0/255.255.255.0",
		"fe80::1%en0", "fe80::1%en0/64", "::ffff:192.0.2.0/129",
	} {
		t.Run(entry, func(t *testing.T) {
			handler, err := restrictAccess(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}), []string{"127.0.0.1", entry})
			if err == nil || handler != nil {
				t.Fatalf("handler = %v, error = %v; want no handler and an error", handler, err)
			}
		})
	}
}

func TestRestrictAccessIgnoresProxyHeaders(t *testing.T) {
	for _, remote := range []string{"192.0.2.7:12345", "127.0.0.1:12345"} {
		t.Run(remote, func(t *testing.T) {
			handler, err := restrictAccess(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}), []string{"127.0.0.1"})
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodGet, "/-/metrics", nil)
			r.RemoteAddr = remote
			r.Header.Set("X-Forwarded-For", "127.0.0.1")
			r.Header.Set("X-Real-IP", "127.0.0.1")
			r.Header.Set("Forwarded", "for=127.0.0.1")
			r.Header.Set("IP_HEADER", "127.0.0.1")
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			wantStatus := http.StatusForbidden
			if remote == "127.0.0.1:12345" {
				wantStatus = http.StatusNoContent
			}
			if w.Code != wantStatus {
				t.Fatalf("status = %d; want %d", w.Code, wantStatus)
			}
		})
	}
}
