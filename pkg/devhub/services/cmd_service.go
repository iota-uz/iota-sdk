package services

import (
	"context"
)

type CmdService struct {
	*BaseService
	Command string
	Args    []string
}

func NewCmdService(name, description, port, command string, args []string) *CmdService {
	return &CmdService{
		BaseService: NewBaseService(name, description, port),
		Command:     command,
		Args:        args,
	}
}

func (s *CmdService) Start(ctx context.Context) error {
	return s.runCommand(ctx, s.Command, s.Args...)
}
