package code

import (
	"context"
	"fmt"
	"net/netip"
)

// staticResolver answers from a table.
type staticResolver map[string][]string

func (r staticResolver) LookupNetIP(_ context.Context, _, host string) ([]netip.Addr, error) {
	ips, ok := r[host]
	if !ok {
		return nil, fmt.Errorf("no such host %s", host)
	}
	out := make([]netip.Addr, 0, len(ips))
	for _, ip := range ips {
		out = append(out, netip.MustParseAddr(ip))
	}
	return out, nil
}
