package services

import (
	"context"
)

type CmdService struct {
	*BaseService
	Command      string
	Args         []string
	Dependencies []string
}

func NewCmdService(name, description, port, command string, args []string) *CmdService {
	return &CmdService{
		BaseService:  NewBaseService(name, description, port),
		Command:      command,
		Args:         args,
		Dependencies: []string{},
	}
}

func (s *CmdService) SetDependencies(deps []string) {
	s.Dependencies = deps
}

func (s *CmdService) GetDependencies() []string {
	return s.Dependencies
}

func (s *CmdService) Start(ctx context.Context) error {
	return s.runCommand(ctx, s.Command, s.Args...)
}
