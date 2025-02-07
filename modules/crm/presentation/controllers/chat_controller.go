package controllers

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/websocket"
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
	wsHandler       *websocket.Hub
	userService     *coreservices.UserService
	templateService *services.MessageTemplateService
	clientService   *services.ClientService
	chatService     *services.ChatService
	basePath        string
}

func NewChatController(app application.Application, basePath string) application.Controller {
	return &ChatController{
		app:             app,
		wsHandler:       websocket.NewHub(),
		userService:     app.Service(coreservices.UserService{}).(*coreservices.UserService),
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
	getRouter.HandleFunc("/search", c.Search).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	getRouter.Handle("/ws", c.wsHandler)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("", c.Create).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}/messages", c.SendMessage).Methods(http.MethodPost)

	c.app.EventPublisher().Subscribe(c.onMessageAdded)
}

func (c *ChatController) onMessageAdded(event *chat.MessagedAddedEvent) {
	log.Printf("ChatController: received message added event: %v", event)
	var locale string
	if event.User != nil {
		locale = string(event.User.UILanguage())
	} else {
		locale = "en"
	}
	ctx := composables.WithLocalizer(
		context.Background(),
		i18n.NewLocalizer(c.app.Bundle(), locale),
	)
	ctx = composables.WithPool(ctx, c.app.DB())
	chatMessages := mapping.MapViewModels(event.Result.Messages(), mappers.MessageToViewModel)
	c.broadcastUpdate(ctx, event.Result.ClientID(), chatMessages)
}

func (c *ChatController) broadcastUpdate(ctx context.Context, clientID uint, messages []*viewmodels.Message) {
	strClientID := strconv.Itoa(int(clientID))

	var buf bytes.Buffer
	if err := chatsui.ChatMessages(messages, strClientID).Render(ctx, &buf); err != nil {
		log.Printf("Error rendering chat messages: %v", err)
		return
	}
	c.wsHandler.Broadcast(buf.Bytes())
}

func (c *ChatController) messageTemplates(ctx context.Context) ([]*viewmodels.MessageTemplate, error) {
	templates, err := c.templateService.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(templates, mappers.MessageTemplateToViewModel), nil
}

func (c *ChatController) chatViewModels(ctx context.Context, params *chat.FindParams) ([]*viewmodels.Chat, error) {
	chatEntities, err := c.chatService.GetPaginated(ctx, params)
	if err != nil {
		return nil, err
	}
	viewModels := make([]*viewmodels.Chat, 0, len(chatEntities))
	for _, chatEntity := range chatEntities {
		clientEntity, err := c.clientService.GetByID(ctx, chatEntity.ClientID())
		if err != nil {
			return nil, err
		}
		viewModels = append(viewModels, mappers.ChatToViewModel(chatEntity, clientEntity))
	}
	return viewModels, nil
}

func (c *ChatController) Search(w http.ResponseWriter, r *http.Request) {
	searchQ := r.URL.Query().Get("Query")
	chatViewModels, err := c.chatViewModels(
		r.Context(),
		&chat.FindParams{
			Search: searchQ,
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.LastMessageAt},
				Ascending: false,
			},
		},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(chatsui.ChatList(chatViewModels)).ServeHTTP(w, r)
}

func (c *ChatController) renderChats(w http.ResponseWriter, r *http.Request) {
	chatViewModels, err := c.chatViewModels(
		r.Context(),
		&chat.FindParams{
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.LastMessageAt},
				Ascending: false,
			},
		},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templHandler := templ.Handler(
		chatsui.Index(&chatsui.IndexPageProps{
			WebsocketURL: c.basePath + "/ws",
			SearchURL:    c.basePath + "/search",
			NewChatURL:   "/crm/chats/new",
			Chats:        chatViewModels,
		}),
		templ.WithStreaming(),
	)
	templHandler.ServeHTTP(w, r)
}

func (c *ChatController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chatID := r.URL.Query().Get("chat_id")

	if chatID == "" {
		c.renderChats(w, r.WithContext(templ.WithChildren(ctx, chatsui.NoSelectedChat())))
		return
	}

	chatIDUint, err := strconv.ParseUint(chatID, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chatEntity, err := c.chatService.GetByID(r.Context(), uint(chatIDUint))
	if errors.Is(err, persistence.ErrChatNotFound) {
		c.renderChats(w, r.WithContext(templ.WithChildren(ctx, chatsui.ChatNotFound())))
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	clientEntity, err := c.clientService.GetByID(r.Context(), chatEntity.ClientID())
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
		Chat:       mappers.ChatToViewModel(chatEntity, clientEntity),
		Templates:  messageTemplates,
	}
	c.renderChats(w, r.WithContext(templ.WithChildren(ctx, chatsui.SelectedChat(props))))
}

func (c *ChatController) GetNew(w http.ResponseWriter, r *http.Request) {
	ctx := templ.WithChildren(
		r.Context(),
		chatsui.NewChat(chatsui.NewChatProps{
			BaseURL:       c.basePath,
			CreateChatURL: c.basePath,
			Phone:         "+1",
			Errors:        map[string]string{},
		}),
	)
	c.renderChats(w, r.WithContext(ctx))
}

func (c *ChatController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&CreateChatDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err = c.clientService.Create(r.Context(), &client.CreateDTO{
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
	shared.HxRedirect(w, r, c.basePath)
}

func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
	chatID, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ch, err := c.chatService.GetByID(r.Context(), chatID)
	if errors.Is(err, persistence.ErrChatNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	clientEntity, err := c.clientService.GetByID(r.Context(), ch.ClientID())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	dto, err := composables.UseForm(&SendMessageDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	chatEntity, err := c.chatService.SendMessage(r.Context(), services.SendMessageDTO{
		ChatID:  chatID,
		Message: dto.Message,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	messageTemplates, err := c.messageTemplates(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.broadcastUpdate(r.Context(), chatEntity.ClientID(), mapping.MapViewModels(chatEntity.Messages(), mappers.MessageToViewModel))
	props := chatsui.SelectedChatProps{
		BaseURL:    c.basePath,
		ClientsURL: "/crm/clients",
		Chat:       mappers.ChatToViewModel(chatEntity, clientEntity),
		Templates:  messageTemplates,
	}
	templ.Handler(chatsui.SelectedChat(props)).ServeHTTP(w, r)
}
