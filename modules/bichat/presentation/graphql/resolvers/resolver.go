package resolvers

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

//go:generate go run github.com/99designs/gqlgen generate

import (
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
)

type Resolver struct {
	app               application.Application
	chatService       services.ChatService
	agentService      services.AgentService
	attachmentService services.AttachmentService
	artifactService   services.ArtifactService
}

func NewResolver(
	app application.Application,
	chatService services.ChatService,
	agentService services.AgentService,
	attachmentService services.AttachmentService,
	artifactService services.ArtifactService,
) *Resolver {
	return &Resolver{
		app:               app,
		chatService:       chatService,
		agentService:      agentService,
		attachmentService: attachmentService,
		artifactService:   artifactService,
	}
}
