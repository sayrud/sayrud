// Copyright 2026 E99p1ant. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package metrics

import (
	"fmt"
	"net/http"
	"net/netip"
	"strings"
)

// restrictAccess parses the whitelist once and checks the connection's peer IP.
// Proxy headers are intentionally ignored because clients can supply them.
func restrictAccess(next http.Handler, whitelist []string) (http.Handler, error) {
	prefixes := make([]netip.Prefix, 0, len(whitelist))
	for _, entry := range whitelist {
		value := strings.TrimSpace(entry)
		if addr, err := netip.ParseAddr(value); err == nil && addr.Zone() == "" {
			addr = addr.Unmap()
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
			continue
		}

		prefix, err := netip.ParsePrefix(value)
		if err != nil || prefix.Addr().Zone() != "" {
			return nil, fmt.Errorf("invalid metrics whitelist entry %q: expected an IP address or CIDR", entry)
		}
		if prefix.Addr().Is4In6() && prefix.Bits() >= 96 {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		prefixes = append(prefixes, prefix.Masked())
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := netip.ParseAddrPort(r.RemoteAddr)
		if err == nil && peer.Addr().Zone() == "" {
			addr := peer.Addr().Unmap()
			for _, prefix := range prefixes {
				if prefix.Contains(addr) {
					next.ServeHTTP(w, r)
					return
				}
			}
		}
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
	}), nil
}
