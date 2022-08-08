// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package resolver

import (
	"fmt"
	"math/rand"
	gonet "net"
	"strconv"
	"strings"

	"github.com/talos-systems/net"
	"google.golang.org/grpc/resolver"
)

// RegisterRoundRobinResolver registers round-robin gRPC resolver for specified port and returns scheme to use in grpc.Dial.
func RegisterRoundRobinResolver(port int) (scheme string) {
	scheme = fmt.Sprintf(roundRobinResolverScheme, port)

	resolver.Register(&roundRobinResolverBuilder{
		port:   port,
		scheme: scheme,
	})

	return
}

const roundRobinResolverScheme = "taloslist-%d"

type roundRobinResolverBuilder struct {
	port   int
	scheme string
}

// Build implements resolver.Builder.
func (b *roundRobinResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	r := &roundRobinResolver{
		target: target,
		cc:     cc,
		port:   b.port,
	}

	if err := r.start(); err != nil {
		return nil, err
	}

	return r, nil
}

// Build implements resolver.Builder.
func (b *roundRobinResolverBuilder) Scheme() string {
	return b.scheme
}

type roundRobinResolver struct {
	target resolver.Target
	cc     resolver.ClientConn
	port   int
}

func (r *roundRobinResolver) start() error {
	var addrs []resolver.Address //nolint:prealloc

	for _, a := range strings.Split(r.target.Endpoint, ",") { //nolint:staticcheck
		addr := net.FormatAddress(a)
		serverName := addr
		if net.AddressContainsPort(addr) {
			var err error
			serverName, _, err = gonet.SplitHostPort(addr)
			if err != nil {
				return fmt.Errorf("failed to derive server name from address %s: %w", addr, err)
			}
		} else {
			addr = gonet.JoinHostPort(addr, strconv.Itoa(r.port))
		}

		addrs = append(addrs, resolver.Address{
			ServerName: serverName,
			Addr:       addr,
		})
	}

	// shuffle the list in case client does just one request
	rand.Shuffle(len(addrs), func(i, j int) {
		addrs[i], addrs[j] = addrs[j], addrs[i]
	})

	serviceConfigJSON := `{
		"loadBalancingConfig": [{
			"round_robin": {}
		}]
	}`

	parsedServiceConfig := r.cc.ParseServiceConfig(serviceConfigJSON)

	if parsedServiceConfig.Err != nil {
		return parsedServiceConfig.Err
	}

	return r.cc.UpdateState(resolver.State{
		Addresses:     addrs,
		ServiceConfig: parsedServiceConfig,
	})
}

// ResolveNow implements resolver.Resolver.
func (r *roundRobinResolver) ResolveNow(o resolver.ResolveNowOptions) {}

// ResolveNow implements resolver.Resolver.
func (r *roundRobinResolver) Close() {}
