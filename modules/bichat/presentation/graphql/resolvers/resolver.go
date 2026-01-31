package resolvers

// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/generated"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/model"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/bichat/domain"
	"github.com/iota-uz/iota-sdk/pkg/bichat/services"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type Resolver struct {
	app          application.Application
	chatService  services.ChatService
	agentService services.AgentService
}

func NewResolver(
	app application.Application,
	chatService services.ChatService,
	agentService services.AgentService,
) *Resolver {
	return &Resolver{
		app:          app,
		chatService:  chatService,
		agentService: agentService,
	}
}

// CreateSession is the resolver for the createSession field.
func (r *mutationResolver) CreateSession(ctx context.Context, title *string) (*model.Session, error) {
	const op serrors.Op = "Resolver.CreateSession"

	// Get tenant and user from context
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	user, _ := composables.UseUser(ctx)
	userID := int64(user.ID())

	// Default title
	t := ""
	if title != nil {
		t = *title
	}

	// Create session via service
	session, err := r.chatService.CreateSession(ctx, tenantID, userID, t)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return toGraphQLSession(session), nil
}

// SendMessage is the resolver for the sendMessage field.
func (r *mutationResolver) SendMessage(ctx context.Context, sessionID string, content string, attachments []*graphql.Upload) (*model.SendMessageResponse, error) {
	panic("not implemented")
}

// ResumeWithAnswer is the resolver for the resumeWithAnswer field.
func (r *mutationResolver) ResumeWithAnswer(ctx context.Context, sessionID string, checkpointID string, answers string) (*model.SendMessageResponse, error) {
	panic("not implemented")
}

// ArchiveSession is the resolver for the archiveSession field.
func (r *mutationResolver) ArchiveSession(ctx context.Context, id string) (*model.Session, error) {
	const op serrors.Op = "Resolver.ArchiveSession"

	sessionID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	session, err := r.chatService.ArchiveSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return toGraphQLSession(session), nil
}

// PinSession is the resolver for the pinSession field.
func (r *mutationResolver) PinSession(ctx context.Context, id string) (*model.Session, error) {
	const op serrors.Op = "Resolver.PinSession"

	sessionID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	session, err := r.chatService.PinSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return toGraphQLSession(session), nil
}

// UnpinSession is the resolver for the unpinSession field.
func (r *mutationResolver) UnpinSession(ctx context.Context, id string) (*model.Session, error) {
	const op serrors.Op = "Resolver.UnpinSession"

	sessionID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	session, err := r.chatService.UnpinSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return toGraphQLSession(session), nil
}

// DeleteSession is the resolver for the deleteSession field.
func (r *mutationResolver) DeleteSession(ctx context.Context, id string) (bool, error) {
	const op serrors.Op = "Resolver.DeleteSession"

	// TODO: Implement DeleteSession in ChatService
	// For now, return not implemented error
	return false, serrors.E(op, "not yet implemented", "delete session not yet implemented")
}

// Sessions is the resolver for the sessions field.
func (r *queryResolver) Sessions(ctx context.Context, limit *int, offset *int) ([]*model.Session, error) {
	const op serrors.Op = "Resolver.Sessions"

	user, _ := composables.UseUser(ctx)
	userID := int64(user.ID())

	// Set defaults
	l := 20
	if limit != nil {
		l = *limit
	}
	o := 0
	if offset != nil {
		o = *offset
	}

	opts := domain.ListOptions{
		Limit:  l,
		Offset: o,
	}

	sessions, err := r.chatService.ListUserSessions(ctx, userID, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert to GraphQL models
	result := make([]*model.Session, len(sessions))
	for i, s := range sessions {
		result[i] = toGraphQLSession(s)
	}

	return result, nil
}

// Session is the resolver for the session field.
func (r *queryResolver) Session(ctx context.Context, id string) (*model.Session, error) {
	const op serrors.Op = "Resolver.Session"

	sessionID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	session, err := r.chatService.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return toGraphQLSession(session), nil
}

// Messages is the resolver for the messages field.
func (r *queryResolver) Messages(ctx context.Context, sessionID string, limit *int, offset *int) ([]*model.Message, error) {
	const op serrors.Op = "Resolver.Messages"

	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	// Set defaults
	l := 50
	if limit != nil {
		l = *limit
	}
	o := 0
	if offset != nil {
		o = *offset
	}

	opts := domain.ListOptions{
		Limit:  l,
		Offset: o,
	}

	messages, err := r.chatService.GetSessionMessages(ctx, sid, opts)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert to GraphQL models
	result := make([]*model.Message, len(messages))
	for i, m := range messages {
		result[i] = toGraphQLMessage(m)
	}

	return result, nil
}

// MessageStream is the resolver for the messageStream field.
func (r *subscriptionResolver) MessageStream(ctx context.Context, sessionID string) (<-chan *model.MessageChunk, error) {
	panic("not implemented")
}

// Mutation returns generated.MutationResolver implementation.
func (r *Resolver) Mutation() generated.MutationResolver { return &mutationResolver{r} }

// Query returns generated.QueryResolver implementation.
func (r *Resolver) Query() generated.QueryResolver { return &queryResolver{r} }

// Subscription returns generated.SubscriptionResolver implementation.
func (r *Resolver) Subscription() generated.SubscriptionResolver { return &subscriptionResolver{r} }

type mutationResolver struct{ *Resolver }
type queryResolver struct{ *Resolver }
type subscriptionResolver struct{ *Resolver }
