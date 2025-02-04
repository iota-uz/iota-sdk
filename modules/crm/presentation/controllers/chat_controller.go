package controllers

import (
	"bytes"
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"sync"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/nicksnyder/go-i18n/v2/i18n"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/client"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/entities/message"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence"
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
	wsHandler       *WebSocketHandler
	messagesService *services.MessagesService
	userService     *coreservices.UserService
	templateService *services.MessageTemplateService
	clientService   *services.ClientService
	chatService     *services.ChatService
	basePath        string
}

func NewChatController(app application.Application, basePath string) application.Controller {
	return &ChatController{
		app:             app,
		wsHandler:       NewWebSocketHandler(),
		messagesService: app.Service(services.MessagesService{}).(*services.MessagesService),
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

	c.app.EventPublisher().Subscribe(c.onNewMessage)
}

func (c *ChatController) onNewMessage(event *message.CreatedEvent) {
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
	chatEntity, err := c.chatService.GetByID(ctx, event.Result.ChatID())
	if err != nil {
		log.Printf("Error getting chat: %v", err)
		return
	}
	clientID := strconv.Itoa(int(chatEntity.Client().ID()))
	chatMessages, err := c.chatMessages(ctx, chatEntity.ID())
	if err != nil {
		log.Printf("Error getting chat messages: %v", err)
		return
	}
	c.broadcastUpdate(ctx, clientID, chatMessages)
}

func (c *ChatController) broadcastUpdate(ctx context.Context, clientID string, messages []*viewmodels.Message) {
	var buf bytes.Buffer
	if err := chatsui.ChatMessages(messages, clientID).Render(ctx, &buf); err != nil {
		log.Printf("Error rendering chat messages: %v", err)
		return
	}
	c.wsHandler.Broadcast(websocket.TextMessage, buf.Bytes())
}

func (c *ChatController) chatMessages(ctx context.Context, chatID uint) ([]*viewmodels.Message, error) {
	messages, err := c.messagesService.GetByChatID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(messages, mappers.MessageToViewModel), nil
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
	viewModels := mapping.MapViewModels(chatEntities, mappers.ChatToViewModel)
	return viewModels, nil
}

func (c *ChatController) Search(w http.ResponseWriter, r *http.Request) {
	searchQ := r.URL.Query().Get("Query")
	chatViewModels, err := c.chatViewModels(
		r.Context(),
		&chat.FindParams{
			Search: searchQ,
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.CreatedAt},
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

func (c *ChatController) List(w http.ResponseWriter, r *http.Request) {
	messageTemplates, err := c.messageTemplates(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	chatViewModels, err := c.chatViewModels(
		r.Context(),
		&chat.FindParams{
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.CreatedAt},
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
	ctx := r.Context()
	chatID := r.URL.Query().Get("chat_id")

	if chatID == "" {
		templHandler.ServeHTTP(
			w, r.WithContext(templ.WithChildren(ctx, chatsui.NoSelectedChat())),
		)
		return
	}

	chatIDuint, err := strconv.ParseUint(chatID, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	for _, chat := range chatViewModels {
		if chat.ID == chatID {
			chatMessages, err := c.chatMessages(ctx, uint(chatIDuint))
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			props := chatsui.SelectedChatProps{
				BaseURL:    c.basePath,
				ClientsURL: "/crm/clients",
				Chat:       chat,
				Templates:  messageTemplates,
				Messages:   chatMessages,
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
}

func (c *ChatController) GetNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	chatViewModels, err := c.chatViewModels(
		r.Context(),
		&chat.FindParams{
			SortBy: chat.SortBy{
				Fields:    []chat.Field{chat.CreatedAt},
				Ascending: false,
			},
		},
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &chatsui.IndexPageProps{
		Chats:      chatViewModels,
		NewChatURL: "/crm/chats",
		SearchURL:  c.basePath + "/search",
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
	chatEntity, err := c.chatService.GetByID(r.Context(), chatID)
	if errors.Is(err, persistence.ErrChatNotFound) {
		http.Error(w, err.Error(), http.StatusNotFound)
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

	_, err = c.messagesService.SendMessage(r.Context(), services.SendMessageDTO{
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
	clientID := strconv.Itoa(int(chatEntity.Client().ID()))
	chatMessages, err := c.chatMessages(r.Context(), chatEntity.ID())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	c.broadcastUpdate(r.Context(), clientID, chatMessages)
	props := chatsui.SelectedChatProps{
		BaseURL:    c.basePath,
		ClientsURL: "/crm/clients",
		Chat:       mappers.ChatToViewModel(chatEntity),
		Messages:   chatMessages,
		Templates:  messageTemplates,
	}
	component := chatsui.SelectedChat(props)
	templ.Handler(component).ServeHTTP(w, r)
}

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	upgrader websocket.Upgrader
	clients  map[*websocket.Conn]bool
	mu       sync.RWMutex
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler() *WebSocketHandler {
	return &WebSocketHandler{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		clients: make(map[*websocket.Conn]bool),
	}
}

// readPump reads messages from the WebSocket connection
func (h *WebSocketHandler) readPump(conn *websocket.Conn) {
	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	for {
		messageType, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Error reading message: %v", err)
			}
			return
		}

		if err := h.handleMessage(conn, messageType, message); err != nil {
			log.Printf("Error handling message: %v", err)
			return
		}
	}
}

// ServeHTTP implements the http.Handler interface
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Error upgrading connection: %v", err)
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	// Start reading messages in a separate goroutine and return
	go h.readPump(conn)
}

// handleMessage processes incoming messages
func (h *WebSocketHandler) handleMessage(_ *websocket.Conn, _ int, _ []byte) error {
	return nil
}

// broadcast sends a message to all connected clients except the sender
func (h *WebSocketHandler) Broadcast(messageType int, message []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		if err := client.WriteMessage(messageType, message); err != nil {
			log.Printf("Error broadcasting message: %v", err)
		}
	}
}
