package cli

import "github.com/iota-uz/iota-sdk/pkg/dbctl/execution"

type Option func(*commandOptions)

type commandOptions struct {
	host execution.Host
}

func WithHost(host execution.Host) Option {
	return func(opts *commandOptions) {
		opts.host = host
	}
}
