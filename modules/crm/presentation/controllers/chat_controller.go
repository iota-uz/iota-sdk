package controllers

import (
	"context"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/chats"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type CreateChatDTO struct {
	Phone string
}

type SendMessageDTO struct {
	Message string
}

type ChatController struct {
	app             application.Application
	templateService *services.MessageTemplateService
	clientService   *services.ClientService
	chatService     *services.ChatService
	basePath        string
}

func NewChatController(app application.Application, basePath string) application.Controller {
	return &ChatController{
		app:             app,
		clientService:   app.Service(services.ClientService{}).(*services.ClientService),
		chatService:     app.Service(services.ChatService{}).(*services.ChatService),
		templateService: app.Service(services.MessageTemplateService{}).(*services.MessageTemplateService),
		basePath:        basePath,
	}
}

func (c *ChatController) Key() string {
	return c.basePath
}

func (c *ChatController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}/messages", c.SendMessage).Methods(http.MethodPost)
}

func (c *ChatController) messageTemplates(ctx context.Context) ([]*viewmodels.MessageTemplate, error) {
	templates, err := c.templateService.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(templates, mappers.MessageTemplateToViewModel), nil
}

func (c *ChatController) List(w http.ResponseWriter, r *http.Request) {
	chatEntities, err := c.chatService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	chatViewModels := mapping.MapViewModels(chatEntities, mappers.ChatToViewModel)
	messageTemplates, err := c.messageTemplates(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &chatsui.IndexPageProps{
		ClientsURL: "/crm/clients",
		NewChatURL: "/crm/chats/new",
		Chats:      chatViewModels,
	}
	templHandler := templ.Handler(
		chatsui.Index(props),
		templ.WithStreaming(),
	)

	ctx := r.Context()
	chatID := r.URL.Query().Get("chat_id")
	if chatID != "" {
		for _, chat := range chatViewModels {
			if chat.ID == chatID {
				props := chatsui.SelectedChatProps{
					BaseURL:    c.basePath,
					ClientsURL: "/crm/clients",
					Chat:       chat,
					Templates:  messageTemplates,
				}
				templHandler.ServeHTTP(
					w, r.WithContext(templ.WithChildren(ctx, chatsui.SelectedChat(props))),
				)
				return
			}
		}
		templHandler.ServeHTTP(
			w, r.WithContext(templ.WithChildren(ctx, chatsui.ChatNotFound())),
		)
	} else {
		templHandler.ServeHTTP(
			w, r.WithContext(templ.WithChildren(ctx, chatsui.NoSelectedChat())),
		)
	}
}

func (c *ChatController) GetNew(w http.ResponseWriter, r *http.Request) {
	chatEntities, err := c.chatService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	chatViewModels := mapping.MapViewModels(chatEntities, mappers.ChatToViewModel)
	ctx := r.Context()
	props := &chatsui.IndexPageProps{
		Chats: chatViewModels,
	}
	templHandler := templ.Handler(chatsui.Index(props), templ.WithStreaming())
	templHandler.ServeHTTP(
		w, r.WithContext(templ.WithChildren(ctx, chatsui.NewChat(chatsui.NewChatProps{
			BaseURL:       c.basePath,
			CreateChatURL: c.basePath,
			Phone:         "+1",
			Errors:        map[string]string{},
		}))),
	)
}

func (c *ChatController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&CreateChatDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	createdClient, err := c.clientService.Create(r.Context(), &client.CreateDTO{
		FirstName: "Unknown",
		LastName:  "Unknown",
		Phone:     dto.Phone,
	})
	if errors.Is(err, phone.ErrInvalidPhoneNumber) {
		templ.Handler(chatsui.NewChatForm(chatsui.NewChatProps{
			BaseURL:       c.basePath,
			CreateChatURL: c.basePath,
			Phone:         dto.Phone,
			Errors: map[string]string{
				"Phone": err.Error(),
			},
		})).ServeHTTP(w, r)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, err = c.chatService.Create(r.Context(), &chat.CreateDTO{
		ClientID: createdClient.ID(),
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.HxRedirect(w, r, c.basePath)
}

func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
	chatID, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	chatEntity, err := c.chatService.GetByID(r.Context(), chatID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	dto, err := composables.UseForm(&SendMessageDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	updatedChat, err := c.chatService.SendMessage(r.Context(), chatEntity.ID(), dto.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	messageTemplates, err := c.messageTemplates(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := chatsui.SelectedChatProps{
		BaseURL:    c.basePath,
		ClientsURL: "/crm/clients",
		Chat:       mappers.ChatToViewModel(updatedChat),
		Templates:  messageTemplates,
	}
	component := chatsui.SelectedChat(props)
	templ.Handler(component).ServeHTTP(w, r)
}
