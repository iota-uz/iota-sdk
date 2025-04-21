package controllers

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strconv"

	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"

	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/mappers"
	chatsui "github.com/iota-uz/iota-sdk/modules/crm/presentation/templates/pages/chats"
	"github.com/iota-uz/iota-sdk/modules/crm/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/crm/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/server"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/types"
)

type CreateChatDTO struct {
	Phone string
}

type SendMessageDTO struct {
	Message string
}

type ChatController struct {
	app             application.Application
	userService     *coreservices.UserService
	templateService *services.MessageTemplateService
	clientService   *services.ClientService
	chatService     *services.ChatService
	basePath        string
}

func NewChatController(app application.Application, basePath string) application.Controller {
	return &ChatController{
		app:             app,
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
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/search", c.Search).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}/messages", c.SendMessage).Methods(http.MethodPost)

	c.app.EventPublisher().Subscribe(c.onMessageAdded)
	c.app.EventPublisher().Subscribe(c.onChatCreated)
}

func (c *ChatController) onChatCreated(_ *chat.CreatedEvent) {
	localizer := i18n.NewLocalizer(c.app.Bundle(), "en")
	ctx := intl.WithLocalizer(
		context.Background(),
		localizer,
	)
	ctx = composables.WithPool(ctx, c.app.DB())
	_url, _ := url.Parse(c.basePath)
	ctx = composables.WithPageCtx(ctx, &types.PageContext{
		URL:       _url,
		Locale:    language.English,
		Localizer: localizer,
	})
	c.broadcastChatsListUpdate(ctx)
}

func (c *ChatController) onMessageAdded(event *chat.MessagedAddedEvent) {
	var locale string
	if event.User != nil {
		locale = string(event.User.UILanguage())
	} else {
		locale = "en"
	}
	localizer := i18n.NewLocalizer(c.app.Bundle(), locale)
	ctx := intl.WithLocalizer(
		context.Background(),
		localizer,
	)
	ctx = composables.WithPool(ctx, c.app.DB())
	_url, _ := url.Parse(c.basePath)
	ctx = composables.WithPageCtx(ctx, &types.PageContext{
		URL:       _url,
		Locale:    language.English,
		Localizer: localizer,
	})

	clientEntity, err := c.clientService.GetByID(ctx, event.Result.ClientID())
	if err != nil {
		log.Printf("Error getting client: %v", err)
		return
	}
	chatViewModel := mappers.ChatToViewModel(event.Result, clientEntity)
	var buf bytes.Buffer
	if err := chatsui.ChatMessages(chatViewModel).Render(ctx, &buf); err != nil {
		log.Printf("Error rendering chat messages: %v", err)
		return
	}
	hub := server.WsHub()
	hub.BroadcastToChannel(server.ChannelChat, buf.Bytes())
	c.broadcastChatsListUpdate(ctx)
}

func (c *ChatController) broadcastChatsListUpdate(ctx context.Context) {
	chatViewModels, err := c.chatViewModels(
		ctx,
		&chat.FindParams{
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.LastMessageAt},
				Ascending: false,
			},
		},
	)
	if err != nil {
		log.Printf("Error rendering chat list: %v", err)
		return
	}

	var buf bytes.Buffer
	if err := chatsui.ChatList(chatViewModels).Render(ctx, &buf); err != nil {
		log.Printf("Error rendering chat list: %v", err)
		return
	}
	hub := server.WsHub()
	hub.BroadcastToChannel(server.ChannelChat, buf.Bytes())
}

func (c *ChatController) messageTemplates(ctx context.Context) ([]*viewmodels.MessageTemplate, error) {
	templates, err := c.templateService.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(templates, mappers.MessageTemplateToViewModel), nil
}

func (c *ChatController) chatViewModels(
	ctx context.Context, params *chat.FindParams,
) ([]*viewmodels.Chat, error) {
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

	props := chatsui.IndexPageProps{
		SearchURL:  c.basePath + "/search",
		NewChatURL: "/crm/chats/new",
		Chats:      chatViewModels,
	}
	var component templ.Component
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	if isHxRequest {
		component = chatsui.ChatLayout(props)
	} else {
		component = chatsui.Index(props)
	}
	templ.Handler(component, templ.WithStreaming()).ServeHTTP(w, r)
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
	chatEntity.MarkAllAsRead()
	chatEntity, err = c.chatService.Update(r.Context(), chatEntity)
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
		FirstName: "",
		LastName:  "",
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
	htmx.Redirect(w, c.basePath)
}

func (c *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
	chatID, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	clientEntity, err := c.clientService.GetByID(r.Context(), chatEntity.ClientID())
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
	templ.Handler(chatsui.SelectedChat(props)).ServeHTTP(w, r)
}
