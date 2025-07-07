package services

import (
	"context"
)

type TunnelService struct {
	*BaseService
}

func NewTunnelService() *TunnelService {
	return &TunnelService{
		BaseService: NewBaseService("Cloudflare Tunnel", "Cloudflare tunnel for local development", ""),
	}
}

func (s *TunnelService) Start(ctx context.Context) error {
	return s.runCommand(ctx, "cloudflared", "tunnel", "--url", "http://localhost:3200", "--loglevel", "debug")
}