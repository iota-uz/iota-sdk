// Package services provides this package.
package services

import (
	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	bichatservices "github.com/iota-uz/iota-sdk/pkg/bichat/services"
)

// ChatApplicationServices exposes explicit command/query facades over shared use cases.
type ChatApplicationServices struct {
	SessionCommands bichatservices.SessionCommands
	SessionQueries  bichatservices.SessionQueries
	TurnCommands    bichatservices.TurnCommands
	TurnQueries     bichatservices.TurnQueries
	StreamCommands  bichatservices.StreamCommands
	HITLCommands    bichatservices.HITLCommands
	Observability   *StreamObservability
	// core holds the single chatServiceImpl so shutdown helpers can reach it.
	core            *chatServiceImpl
}

type sessionCommandsService struct{ *chatServiceImpl }
type sessionQueriesService struct{ *chatServiceImpl }
type turnCommandsService struct{ *chatServiceImpl }
type turnQueriesService struct{ *chatServiceImpl }
type streamCommandsService struct{ *chatServiceImpl }
type hitlCommandsService struct{ *chatServiceImpl }

// NewChatApplicationServices builds command/query service facades.
func NewChatApplicationServices(
	chatRepo domain.ChatRepository,
	agentService bichatservices.AgentService,
	model agents.Model,
	titleService TitleService,
	titleQueue TitleJobQueue,
) ChatApplicationServices {
	core := NewChatService(chatRepo, agentService, model, titleService, titleQueue)
	return ChatApplicationServices{
		SessionCommands: &sessionCommandsService{chatServiceImpl: core},
		SessionQueries:  &sessionQueriesService{chatServiceImpl: core},
		TurnCommands:    &turnCommandsService{chatServiceImpl: core},
		TurnQueries:     &turnQueriesService{chatServiceImpl: core},
		StreamCommands:  &streamCommandsService{chatServiceImpl: core},
		HITLCommands:    &hitlCommandsService{chatServiceImpl: core},
		Observability:   NewStreamObservability(core.runRegistry),
		core:            core,
	}
}

// CloseSharedRedis releases the shared *redis.Client that backs all Redis
// components. Must be called exactly once at shutdown; no-op when Redis is
// unconfigured.
func (s *ChatApplicationServices) CloseSharedRedis() error {
	if s.core == nil {
		return nil
	}
	return s.core.CloseSharedRedis()
}
