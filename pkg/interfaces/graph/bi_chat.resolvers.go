package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.
// Code generated by github.com/99designs/gqlgen version v0.17.55

import (
	"context"
	"errors"
	"fmt"

	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/domain/entities/dialogue"
	model "github.com/iota-agency/iota-sdk/pkg/interfaces/graph/gqlmodels"
	"github.com/iota-agency/iota-sdk/pkg/services"
)

// NewDialogue is the resolver for the newDialogue field.
func (r *mutationResolver) NewDialogue(ctx context.Context, input model.NewDialogue) (*model.Dialogue, error) {
	if !composables.UseAuthenticated(ctx) {
		return nil, errors.New("authentication required")
	}
	openaiModel := "gpt-4o-2024-05-13"
	if input.Model != nil {
		openaiModel = *input.Model
	}
	dialogueService := r.app.Service(services.DialogueService{}).(*services.DialogueService)
	data, err := dialogueService.StartDialogue(ctx, input.Message, openaiModel)
	if err != nil {
		return nil, err
	}
	return data.ToGraph()
}

// ReplyDialogue is the resolver for the replyDialogue field.
func (r *mutationResolver) ReplyDialogue(ctx context.Context, id int64, input model.DialogueReply) (*model.Dialogue, error) {
	if !composables.UseAuthenticated(ctx) {
		return nil, errors.New("authentication required")
	}
	openaiModel := "gpt-4o-2024-05-13"
	if input.Model != nil {
		openaiModel = *input.Model
	}
	dialogueService := r.app.Service(services.DialogueService{}).(*services.DialogueService)
	data, err := dialogueService.ReplyToDialogue(ctx, id, input.Message, openaiModel)
	if err != nil {
		return nil, err
	}
	return data.ToGraph()
}

// DeleteDialogue is the resolver for the deleteDialogue field.
func (r *mutationResolver) DeleteDialogue(ctx context.Context, id int64) (*model.Dialogue, error) {
	dialogueService := r.app.Service(services.DialogueService{}).(*services.DialogueService)
	entity, err := dialogueService.Delete(ctx, id)
	if err != nil {
		return nil, err
	}
	return entity.ToGraph()
}

// UpdatePrompt is the resolver for the updatePrompt field.
func (r *mutationResolver) UpdatePrompt(ctx context.Context, id string, input model.UpdatePrompt) (*model.Prompt, error) {
	panic(fmt.Errorf("not implemented: Update - updatePrompt"))
}

// Dialogue is the resolver for the dialogue field.
func (r *queryResolver) Dialogue(ctx context.Context, id int64) (*model.Dialogue, error) {
	dialogueService := r.app.Service(services.DialogueService{}).(*services.DialogueService)
	entity, err := dialogueService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return entity.ToGraph()
}

// Dialogues is the resolver for the dialogues field.
func (r *queryResolver) Dialogues(ctx context.Context, offset int, limit int, sortBy []string) (*model.PaginatedDialogues, error) {
	dialogueService := r.app.Service(services.DialogueService{}).(*services.DialogueService)
	entities, err := dialogueService.GetPaginated(ctx, limit, offset, sortBy)
	if err != nil {
		return nil, err
	}
	result := make([]*model.Dialogue, len(entities))
	for i, entity := range entities {
		r, err := entity.ToGraph()
		if err != nil {
			return nil, err
		}
		result[i] = r
	}
	userService := r.app.Service(services.UserService{}).(*services.UserService)
	total, err := userService.Count(ctx)
	if err != nil {
		return nil, err
	}
	return &model.PaginatedDialogues{
		Data:  result,
		Total: total,
	}, nil
}

// Prompt is the resolver for the prompt field.
func (r *queryResolver) Prompt(ctx context.Context, id string) (*model.Prompt, error) {
	promptService := r.app.Service(services.PromptService{}).(*services.PromptService)
	entity, err := promptService.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return entity.ToGraph(), nil
}

// Prompts is the resolver for the prompts field.
func (r *queryResolver) Prompts(ctx context.Context, offset int, limit int, sortBy []string) (*model.PaginatedPrompts, error) {
	promptService := r.app.Service(services.PromptService{}).(*services.PromptService)
	entities, err := promptService.GetPaginated(ctx, limit, offset, sortBy)
	if err != nil {
		return nil, err
	}
	result := make([]*model.Prompt, len(entities))
	for i, entity := range entities {
		result[i] = entity.ToGraph()
	}
	total, err := promptService.Count(ctx)
	if err != nil {
		return nil, err
	}
	return &model.PaginatedPrompts{
		Data:  result,
		Total: total,
	}, nil
}

// DialogueCreated is the resolver for the dialogueCreated field.
func (r *subscriptionResolver) DialogueCreated(ctx context.Context) (<-chan *model.Dialogue, error) {
	ch := make(chan *model.Dialogue)
	r.app.EventPublisher().Subscribe(func(evt *dialogue.CreatedEvent) {
		res, err := evt.Result.ToGraph()
		if err == nil {
			ch <- res
		}
	})
	return ch, nil
}

// DialogueUpdated is the resolver for the dialogueUpdated field.
func (r *subscriptionResolver) DialogueUpdated(ctx context.Context) (<-chan *model.Dialogue, error) {
	ch := make(chan *model.Dialogue)
	r.app.EventPublisher().Subscribe(func(evt *dialogue.UpdatedEvent) {
		res, err := evt.Result.ToGraph()
		if err == nil {
			ch <- res
		}
	})
	return ch, nil
}

// DialogueDeleted is the resolver for the dialogueDeleted field.
func (r *subscriptionResolver) DialogueDeleted(ctx context.Context) (<-chan *model.Dialogue, error) {
	ch := make(chan *model.Dialogue)
	r.app.EventPublisher().Subscribe(func(evt *dialogue.DeletedEvent) {
		res, err := evt.Result.ToGraph()
		if err == nil {
			ch <- res
		}
	})
	return ch, nil
}

// PromptUpdated is the resolver for the promptUpdated field.
func (r *subscriptionResolver) PromptUpdated(ctx context.Context) (<-chan *model.Prompt, error) {
	panic(fmt.Errorf("not implemented: PromptUpdated - promptUpdated"))
}