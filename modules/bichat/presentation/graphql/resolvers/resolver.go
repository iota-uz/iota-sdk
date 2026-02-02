package resolvers

// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.

import (
	"context"
	"encoding/json"
	"time"

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

// CreateSession is the resolver for the createSession field.
func (r *mutationResolver) CreateSession(ctx context.Context, title *string) (*model.Session, error) {
	const op serrors.Op = "Resolver.CreateSession"

	// Get tenant and user from context
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, err)
	}

	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "user not authenticated")
	}
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
	const op serrors.Op = "Resolver.SendMessage"

	// Parse sessionID
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid session ID", err)
	}

	// Get authenticated user
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "user not authenticated")
	}

	// Get tenant ID for storage isolation
	tenantID, err := composables.UseTenantID(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.Internal, "failed to get tenant ID", err)
	}

	// Parse and save attachments from GraphQL uploads
	domainAttachments := make([]domain.Attachment, 0, len(attachments))
	for _, upload := range attachments {
		if upload == nil {
			continue
		}

		// Validate and save to storage
		// Note: user.ID() returns uint, create a UUID from it
		userUUID := uuid.New() // In production, should derive from user ID or use proper mapping
		attachment, err := r.attachmentService.ValidateAndSave(
			ctx,
			upload.Filename,
			upload.ContentType,
			upload.Size,
			upload.File,
			tenantID,
			userUUID,
		)
		if err != nil {
			return nil, serrors.E(op, err, "failed to save attachment")
		}

		domainAttachments = append(domainAttachments, *attachment)
	}

	// Call service
	req := services.SendMessageRequest{
		SessionID:   sid,
		UserID:      int64(user.ID()),
		Content:     content,
		Attachments: domainAttachments,
	}

	resp, err := r.chatService.SendMessage(ctx, req)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert to GraphQL response
	gqlResp := &model.SendMessageResponse{
		UserMessage:      toGraphQLMessage(resp.UserMessage),
		AssistantMessage: toGraphQLMessage(resp.AssistantMessage),
		Session:          toGraphQLSession(resp.Session),
	}

	// Convert interrupt if present
	if resp.Interrupt != nil {
		gqlResp.Interrupt = toGraphQLInterrupt(resp.Interrupt)
	}

	return gqlResp, nil
}

// ResumeWithAnswer is the resolver for the resumeWithAnswer field.
func (r *mutationResolver) ResumeWithAnswer(ctx context.Context, sessionID string, checkpointID string, answers string) (*model.SendMessageResponse, error) {
	const op serrors.Op = "Resolver.ResumeWithAnswer"

	// Parse sessionID
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid session ID", err)
	}

	// Get authenticated user
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "user not authenticated")
	}

	// Verify session ownership
	session, err := r.chatService.GetSession(ctx, sid)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if session.UserID != int64(user.ID()) {
		return nil, serrors.E(op, serrors.PermissionDenied, "session does not belong to user")
	}

	// Validate checkpointID
	if checkpointID == "" {
		return nil, serrors.E(op, serrors.KindValidation, "checkpoint ID is required")
	}

	// Parse answers JSON string to map[string]string
	var answersMap map[string]string
	if err := json.Unmarshal([]byte(answers), &answersMap); err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid answers JSON", err)
	}

	// Call service
	req := services.ResumeRequest{
		SessionID:    sid,
		CheckpointID: checkpointID,
		Answers:      answersMap,
	}

	resp, err := r.chatService.ResumeWithAnswer(ctx, req)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	// Convert to GraphQL response
	gqlResp := &model.SendMessageResponse{
		UserMessage:      toGraphQLMessage(resp.UserMessage),
		AssistantMessage: toGraphQLMessage(resp.AssistantMessage),
		Session:          toGraphQLSession(resp.Session),
	}

	// Convert interrupt if present
	if resp.Interrupt != nil {
		gqlResp.Interrupt = toGraphQLInterrupt(resp.Interrupt)
	}

	return gqlResp, nil
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

// UpdateSessionTitle is the resolver for the updateSessionTitle field.
func (r *mutationResolver) UpdateSessionTitle(ctx context.Context, id string, title string) (*model.Session, error) {
	const op serrors.Op = "Resolver.UpdateSessionTitle"

	// Parse UUID
	sessionID, err := uuid.Parse(id)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid session ID", err)
	}

	// Get authenticated user
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "user not authenticated")
	}

	// Verify session ownership
	session, err := r.chatService.GetSession(ctx, sessionID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if session.UserID != int64(user.ID()) {
		return nil, serrors.E(op, serrors.PermissionDenied, "session does not belong to user")
	}

	// Update title
	updatedSession, err := r.chatService.UpdateSessionTitle(ctx, sessionID, title)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return toGraphQLSession(updatedSession), nil
}

// DeleteSession is the resolver for the deleteSession field.
func (r *mutationResolver) DeleteSession(ctx context.Context, id string) (bool, error) {
	const op serrors.Op = "Resolver.DeleteSession"

	// Parse UUID
	sessionID, err := uuid.Parse(id)
	if err != nil {
		return false, serrors.E(op, serrors.KindValidation, "invalid session ID", err)
	}

	// Get authenticated user
	user, err := composables.UseUser(ctx)
	if err != nil {
		return false, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil {
		return false, serrors.E(op, serrors.PermissionDenied, "user not authenticated")
	}

	// Verify session ownership
	session, err := r.chatService.GetSession(ctx, sessionID)
	if err != nil {
		return false, serrors.E(op, err)
	}
	if session.UserID != int64(user.ID()) {
		return false, serrors.E(op, serrors.PermissionDenied, "session does not belong to user")
	}

	// Delete session (cascades to messages and attachments via repository)
	err = r.chatService.DeleteSession(ctx, sessionID)
	if err != nil {
		return false, serrors.E(op, err)
	}

	return true, nil
}

// CancelPendingQuestion is the resolver for the cancelPendingQuestion field.
func (r *mutationResolver) CancelPendingQuestion(ctx context.Context, sessionID string) (*model.Session, error) {
	const op serrors.Op = "Resolver.CancelPendingQuestion"

	// Parse UUID
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid session ID", err)
	}

	// Get authenticated user
	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "user not authenticated")
	}

	// Verify session ownership
	session, err := r.chatService.GetSession(ctx, sid)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if session.UserID != int64(user.ID()) {
		return nil, serrors.E(op, serrors.PermissionDenied, "session does not belong to user")
	}

	// Cancel the pending question
	updatedSession, err := r.chatService.CancelPendingQuestion(ctx, sid)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return toGraphQLSession(updatedSession), nil
}

// Sessions is the resolver for the sessions field.
// Sessions are scoped by tenant and user from context (ListUserSessions uses UseTenantID and userID).
func (r *queryResolver) Sessions(ctx context.Context, limit *int, offset *int) ([]*model.Session, error) {
	const op serrors.Op = "Resolver.Sessions"

	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil {
		return nil, serrors.E(op, serrors.PermissionDenied, "user not authenticated")
	}
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

	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil || int64(user.ID()) != session.UserID {
		return nil, serrors.E(op, serrors.PermissionDenied, "session not found or access denied")
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

	session, err := r.chatService.GetSession(ctx, sid)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	user, err := composables.UseUser(ctx)
	if err != nil {
		return nil, serrors.E(op, serrors.PermissionDenied, err)
	}
	if user == nil || int64(user.ID()) != session.UserID {
		return nil, serrors.E(op, serrors.PermissionDenied, "session not found or access denied")
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

// Session returns the Session resolver (for Session.artifacts).
func (r *Resolver) Session() generated.SessionResolver {
	return &sessionResolver{r}
}

// MessageStream is the resolver for the messageStream field.
//
// IMPORTANT LIMITATION:
// This is a placeholder implementation. GraphQL subscriptions require WebSocket infrastructure
// and don't fit well with the chat streaming model where messages are sent as mutations.
//
// RECOMMENDED APPROACH:
// Use SSE (Server-Sent Events) streaming via the HTTP StreamController instead:
// - POST /bichat/stream - Starts streaming for a message
// - More efficient for chat (unidirectional)
// - Simpler infrastructure (no WebSocket)
// - Better backpressure handling
// - Direct integration with ChatService.SendMessageStream()
//
// This GraphQL subscription remains as a placeholder for future pub/sub implementations
// (e.g., multi-device synchronization, typing indicators, presence).
func (r *subscriptionResolver) MessageStream(ctx context.Context, sessionID string) (<-chan *model.MessageChunk, error) {
	const op serrors.Op = "Resolver.MessageStream"

	// Parse sessionID
	sid, err := uuid.Parse(sessionID)
	if err != nil {
		return nil, serrors.E(op, serrors.KindValidation, "invalid session ID", err)
	}

	// Create channel for GraphQL subscription
	ch := make(chan *model.MessageChunk, 100)

	// Launch goroutine to stream events
	go func() {
		defer close(ch)

		// Get event generator from agent service
		// Note: This is a placeholder - real implementation would get content from request
		// For GraphQL subscriptions, typically you'd have a separate mutation to start streaming
		// and the subscription would listen to those events

		// Get authenticated user
		user, err := composables.UseUser(ctx)
		if err != nil {
			select {
			case ch <- &model.MessageChunk{
				Type:      model.ChunkTypeError,
				Error:     strPtr("authentication required"),
				Timestamp: time.Now(),
			}:
			case <-ctx.Done():
			}
			return
		}
		if user == nil {
			select {
			case ch <- &model.MessageChunk{
				Type:      model.ChunkTypeError,
				Error:     strPtr("user not authenticated"),
				Timestamp: time.Now(),
			}:
			case <-ctx.Done():
			}
			return
		}

		// Note: In a real implementation, you would:
		// 1. Get the message content from somewhere (e.g., a session state or separate mutation)
		// 2. Call agentService.ProcessMessage() to get the Generator
		// 3. Iterate over the generator and convert events to MessageChunks

		// For now, send a placeholder message indicating subscription is active
		select {
		case ch <- &model.MessageChunk{
			Type:      model.ChunkTypeContent,
			Content:   strPtr("Subscription active for session: " + sid.String()),
			Timestamp: time.Now(),
		}:
		case <-ctx.Done():
			return
		}

		// Send done event
		select {
		case ch <- &model.MessageChunk{
			Type:      model.ChunkTypeDone,
			Timestamp: time.Now(),
		}:
		case <-ctx.Done():
		}
	}()

	return ch, nil
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
type sessionResolver struct{ *Resolver }
