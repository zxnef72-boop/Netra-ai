package main

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Force Go pakai DNS public karena /etc/resolv.conf gak ada di Android/NeferOS.
// Tanpa ini, semua HTTP request di Go bakal gagal dengan "no such host".
func init() {
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			servers := []string{
				"8.8.8.8:53",
				"8.8.4.4:53",
				"1.1.1.1:53",
				"1.0.0.1:53",
			}
			var lastErr error
			for _, s := range servers {
				conn, err := d.DialContext(ctx, "udp", s)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, fmt.Errorf("no DNS server reachable: %v", lastErr)
		},
	}
}
