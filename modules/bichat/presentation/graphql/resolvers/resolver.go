package resolvers

// THIS CODE WILL BE UPDATED WITH SCHEMA CHANGES. PREVIOUS IMPLEMENTATION FOR SCHEMA CHANGES WILL BE KEPT IN THE COMMENT SECTION. IMPLEMENTATION FOR UNCHANGED SCHEMA WILL BE KEPT.

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/generated"
	"github.com/iota-uz/iota-sdk/modules/bichat/presentation/graphql/model"
)

type Resolver struct{}

// CreateSession is the resolver for the createSession field.
func (r *mutationResolver) CreateSession(ctx context.Context, title *string) (*model.Session, error) {
	panic("not implemented")
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
	panic("not implemented")
}

// PinSession is the resolver for the pinSession field.
func (r *mutationResolver) PinSession(ctx context.Context, id string) (*model.Session, error) {
	panic("not implemented")
}

// UnpinSession is the resolver for the unpinSession field.
func (r *mutationResolver) UnpinSession(ctx context.Context, id string) (*model.Session, error) {
	panic("not implemented")
}

// UpdateSessionTitle is the resolver for the updateSessionTitle field.
func (r *mutationResolver) UpdateSessionTitle(ctx context.Context, id string, title string) (*model.Session, error) {
	panic("not implemented")
}

// DeleteSession is the resolver for the deleteSession field.
func (r *mutationResolver) DeleteSession(ctx context.Context, id string) (bool, error) {
	panic("not implemented")
}

// CancelPendingQuestion is the resolver for the cancelPendingQuestion field.
func (r *mutationResolver) CancelPendingQuestion(ctx context.Context, sessionID string) (*model.Session, error) {
	panic("not implemented")
}

// DeleteArtifact is the resolver for the deleteArtifact field.
func (r *mutationResolver) DeleteArtifact(ctx context.Context, id string) (bool, error) {
	panic("not implemented")
}

// UpdateArtifact is the resolver for the updateArtifact field.
func (r *mutationResolver) UpdateArtifact(ctx context.Context, id string, name *string, description *string) (*model.Artifact, error) {
	panic("not implemented")
}

// Sessions is the resolver for the sessions field.
func (r *queryResolver) Sessions(ctx context.Context, limit *int, offset *int) ([]*model.Session, error) {
	panic("not implemented")
}

// Session is the resolver for the session field.
func (r *queryResolver) Session(ctx context.Context, id string) (*model.Session, error) {
	panic("not implemented")
}

// Messages is the resolver for the messages field.
func (r *queryResolver) Messages(ctx context.Context, sessionID string, limit *int, offset *int) ([]*model.Message, error) {
	panic("not implemented")
}

// Artifact is the resolver for the artifact field.
func (r *queryResolver) Artifact(ctx context.Context, id string) (*model.Artifact, error) {
	panic("not implemented")
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
