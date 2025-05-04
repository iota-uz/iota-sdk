package persistence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-faster/errors"

	coremodels "github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/modules/crm/domain/aggregates/chat"
	"github.com/iota-uz/iota-sdk/modules/crm/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

var (
	ErrChatNotFound    = errors.New("chat not found")
	ErrMessageNotFound = errors.New("message not found")
	ErrMemberNotFound  = errors.New("member not found")
)

const (
	selectChatQuery = `
		SELECT 
			c.id,
			c.created_at,
			c.last_message_at,
			c.client_id
		FROM chats c
	`

	countChatQuery = `SELECT COUNT(*) as count FROM chats`

	insertChatQuery = `
		INSERT INTO chats (
			client_id,
			created_at
		) VALUES ($1, $2) RETURNING id
	`

	updateChatQuery = `UPDATE chats SET
		client_id = $1,
		created_at = $2,
		last_message_at = $3
		WHERE id = $4`

	deleteChatQuery = `DELETE FROM chats WHERE id = $1`

	selectMessagesQuery = `
		SELECT 
			m.id,
			m.chat_id,
			m.message,
			m.sender_id,
			m.read_at,
			m.sent_at,
			m.created_at
		FROM messages m
	`

	selectMessageAttachmentsQuery = `
		SELECT 
			u.id AS upload_id,
			u.hash,
			u.path,
			u.size,
			u.mimetype,
			u.created_at AS upload_created_at,
			u.updated_at AS upload_updated_at
		FROM messages AS m
		JOIN message_media AS mm ON m.id = mm.message_id
		JOIN uploads AS u ON mm.upload_id = u.id
		WHERE m.id = $1
	`

	selectChatMembersQuery = `
		SELECT 
			cm.id,
			cm.chat_id,
			cm.user_id,
			cm.client_id,
			cm.client_contact_id,
			cm.transport,
			cm.transport_meta,
			cm.created_at,
			cm.updated_at
		FROM chat_members cm
	`

	insertChatMemberQuery = `
		INSERT INTO chat_members (
			id,
			chat_id,
			user_id,
			client_id,
			client_contact_id,
			transport,
			transport_meta,
			created_at,
			updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	deleteChatMembersQuery = `DELETE FROM chat_members WHERE chat_id = $1`

	insertMessageQuery = `
		INSERT INTO messages (
			chat_id,
			message,
			read_at,
			sent_at,
			sender_id,
			created_at
		) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`

	updateMessageQuery = `
		UPDATE messages SET
			chat_id = $1,
			message = $2,
			read_at = $3,
			sender_id = $4,
			sent_at = $5
		WHERE id = $6
	`

	deleteMessageQuery = `DELETE FROM messages WHERE chat_id = $1`
)

type ChatRepository struct {
}

func NewChatRepository() chat.Repository {
	return &ChatRepository{}
}

func (g *ChatRepository) queryMembers(ctx context.Context, query string, args ...any) ([]chat.Member, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute chat members query")
	}
	defer rows.Close()

	var dbMembers []*models.ChatMember
	for rows.Next() {
		var member models.ChatMember
		var transportMetaData []byte
		if err := rows.Scan(
			&member.ID,
			&member.ChatID,
			&member.UserID,
			&member.ClientID,
			&member.ClientContactID,
			&member.Transport,
			&transportMetaData,
			&member.CreatedAt,
			&member.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan chat member")
		}

		// Handle transport metadata based on transport type
		if len(transportMetaData) > 0 {
			member.TransportMeta = models.NewTransportMeta(string(transportMetaData))
		}

		dbMembers = append(dbMembers, &member)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error occurred while iterating chat member rows")
	}

	members := make([]chat.Member, 0, len(dbMembers))
	for _, m := range dbMembers {
		domainMember, err := ToDomainChatMember(m)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain chat member")
		}
		members = append(members, domainMember)
	}

	return members, nil
}

func (g *ChatRepository) queryChats(ctx context.Context, query string, args ...interface{}) ([]chat.Chat, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute chat query")
	}
	defer rows.Close()

	dbChats := make([]*models.Chat, 0)
	for rows.Next() {
		var c models.Chat
		if err := rows.Scan(
			&c.ID,
			&c.CreatedAt,
			&c.LastMessageAt,
			&c.ClientID,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan chat")
		}
		dbChats = append(dbChats, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error occurred while iterating chat rows")
	}

	chats := make([]chat.Chat, 0, len(dbChats))

	for _, c := range dbChats {
		messages, err := g.queryMessages(
			ctx,
			repo.Join(
				selectMessagesQuery,
				"WHERE m.chat_id = $1",
				"ORDER BY m.created_at ASC",
			),
			c.ID,
		)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get messages for chat")
		}

		members, err := g.queryMembers(ctx, selectChatMembersQuery+" WHERE cm.chat_id = $1", c.ID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get members for chat")
		}

		domainChat, err := ToDomainChat(c, messages, members)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain chat")
		}
		chats = append(chats, domainChat)
	}

	return chats, nil
}

func (g *ChatRepository) queryMessages(ctx context.Context, query string, args ...interface{}) ([]chat.Message, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute message query")
	}
	defer rows.Close()

	var dbMessages []*models.Message
	for rows.Next() {
		var msg models.Message
		if err := rows.Scan(
			&msg.ID,
			&msg.ChatID,
			&msg.Message,
			&msg.SenderID,
			&msg.ReadAt,
			&msg.SentAt,
			&msg.CreatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "failed to scan message")
		}

		dbMessages = append(dbMessages, &msg)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error occurred while iterating message rows")
	}

	messages := make([]chat.Message, 0, len(dbMessages))
	for _, message := range dbMessages {
		uploads, err := pool.Query(ctx, selectMessageAttachmentsQuery, message.ID)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to query attachments for message %d", message.ID)
		}
		defer uploads.Close()

		var dbUploads []*coremodels.Upload
		for uploads.Next() {
			var upload coremodels.Upload
			if err := uploads.Scan(
				&upload.ID,
				&upload.Hash,
				&upload.Path,
				&upload.Size,
				&upload.Mimetype,
				&upload.CreatedAt,
				&upload.UpdatedAt,
			); err != nil {
				return nil, errors.Wrap(err, "failed to scan upload")
			}
			dbUploads = append(dbUploads, &upload)
		}

		members, err := g.queryMembers(ctx, selectChatMembersQuery+" WHERE cm.id = $1", message.SenderID)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get members for message")
		}

		if len(members) == 0 {
			return nil, errors.Wrapf(err, "failed to get members for message %d", message.ID)
		}

		if err := uploads.Err(); err != nil {
			return nil, errors.Wrap(err, "error occurred while iterating uploads")
		}

		domainMessage, err := ToDomainMessage(message, members[0], dbUploads)
		if err != nil {
			return nil, errors.Wrap(err, "failed to convert to domain message")
		}
		messages = append(messages, domainMessage)
	}

	return messages, nil
}

func (g *ChatRepository) GetPaginated(
	ctx context.Context, params *chat.FindParams,
) ([]chat.Chat, error) {
	sortFields := []string{}
	for _, f := range params.SortBy.Fields {
		switch f {
		case chat.LastMessageAt:
			sortFields = append(sortFields, "c.last_message_at")
		case chat.CreatedAt:
			sortFields = append(sortFields, "c.created_at")
		default:
			return nil, fmt.Errorf("unknown sort field: %v", f)
		}
	}

	where, args := []string{"1 = 1"}, []interface{}{}
	if params.Search != "" {
		where = append(
			where,
			"cl.first_name ILIKE $1 OR cl.last_name ILIKE $1 OR cl.middle_name ILIKE $1 OR cl.phone_number ILIKE $1",
		)
		args = append(args, "%"+params.Search+"%")
	}
	return g.queryChats(
		ctx,
		repo.Join(
			selectChatQuery,
			repo.JoinWhere(where...),
			repo.OrderBy(sortFields, params.SortBy.Ascending),
			repo.FormatLimitOffset(params.Limit, params.Offset),
		),
		args...,
	)
}

func (g *ChatRepository) Count(ctx context.Context) (int64, error) {
	pool, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}
	var count int64
	if err := pool.QueryRow(ctx, countChatQuery).Scan(&count); err != nil {
		return 0, errors.Wrap(err, "failed to count chats")
	}
	return count, nil
}

func (g *ChatRepository) GetAll(ctx context.Context) ([]chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all chats")
	}
	return chats, nil
}

func (g *ChatRepository) GetByID(ctx context.Context, id uint) (chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery+" WHERE c.id = $1", id)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get chat with id %d", id)
	}
	if len(chats) == 0 {
		return nil, ErrChatNotFound
	}
	return chats[0], nil
}

func (g *ChatRepository) GetByClientID(ctx context.Context, clientID uint) (chat.Chat, error) {
	chats, err := g.queryChats(ctx, selectChatQuery+" WHERE c.client_id = $1", clientID)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get chat for client %d", clientID)
	}
	if len(chats) == 0 {
		return nil, ErrChatNotFound
	}
	return chats[0], nil
}

func (g *ChatRepository) GetMemberByContact(ctx context.Context, contactType string, contactValue string) (chat.Member, error) {
	query := `
		SELECT 
			cm.id,
			cm.chat_id,
			cm.user_id,
			cm.client_id,
			cm.client_contact_id,
			cm.transport,
			cm.transport_meta,
			cm.created_at,
			cm.updated_at
		FROM chat_members cm
		JOIN client_contacts cc ON cm.client_contact_id = cc.id
		WHERE cc.contact_type = $1 AND cc.contact_value = $2
		LIMIT 1
	`

	members, err := g.queryMembers(ctx, query, contactType, contactValue)
	if err != nil {
		return nil, errors.Wrap(err, "failed to query member by contact")
	}

	if len(members) == 0 {
		return nil, ErrMemberNotFound
	}

	return members[0], nil
}

func (g *ChatRepository) insertMessage(ctx context.Context, message *models.Message) (uint, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get transaction")
	}

	// Verify that the sender exists in the database
	var senderExists bool
	err = tx.QueryRow(
		ctx,
		"SELECT EXISTS(SELECT 1 FROM chat_members WHERE id = $1)",
		message.SenderID,
	).Scan(&senderExists)

	if err != nil {
		return 0, errors.Wrap(err, "failed to check if sender exists")
	}

	if !senderExists {
		return 0, errors.Errorf("sender with ID %s does not exist in the database", message.SenderID)
	}

	if err := tx.QueryRow(
		ctx,
		insertMessageQuery,
		message.ChatID,
		message.Message,
		message.ReadAt,
		message.SentAt,
		message.SenderID,
		message.CreatedAt,
	).Scan(&message.ID); err != nil {
		return 0, errors.Wrap(err, "failed to insert message")
	}
	return message.ID, nil
}

func (g *ChatRepository) updateMessage(ctx context.Context, message *models.Message) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	if _, err := tx.Exec(
		ctx,
		updateMessageQuery,
		message.ChatID,
		message.Message,
		message.ReadAt,
		message.SenderID,
		message.SentAt,
		message.ID,
	); err != nil {
		return errors.Wrap(err, "failed to update message")
	}
	return nil
}

func (g *ChatRepository) insertChatMember(ctx context.Context, chatID uint, member *models.ChatMember) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	var exists bool
	err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chat_members WHERE id = $1)", member.ID).Scan(&exists)
	if err != nil {
		return errors.Wrap(err, "failed to check if member exists")
	}

	if exists {
		return nil
	}

	if member.ClientContactID.Valid {
		var existsByContactID bool
		err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM chat_members WHERE client_contact_id = $1 AND chat_id = $2)",
			member.ClientContactID.Int32, chatID).Scan(&existsByContactID)
		if err != nil {
			return errors.Wrap(err, "failed to check if member exists by contact ID")
		}

		if existsByContactID {
			var existingMemberID string
			err = tx.QueryRow(ctx, "SELECT id FROM chat_members WHERE client_contact_id = $1 AND chat_id = $2",
				member.ClientContactID.Int32, chatID).Scan(&existingMemberID)
			if err != nil {
				return errors.Wrap(err, "failed to get existing member ID")
			}

			member.ID = existingMemberID
			return nil
		}
	}

	var transportMeta []byte
	if member.TransportMeta != nil {
		transportMeta, err = json.Marshal(member.TransportMeta.Interface())
		if err != nil {
			return errors.Wrap(err, "failed to marshal transport meta")
		}
	}

	_, err = tx.Exec(
		ctx,
		insertChatMemberQuery,
		member.ID,
		chatID,
		member.UserID,
		member.ClientID,
		member.ClientContactID,
		member.Transport,
		transportMeta,
		member.CreatedAt,
		member.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(err, "failed to insert chat member")
	}
	return nil
}

func (g *ChatRepository) saveMembers(ctx context.Context, chatID uint, members []chat.Member) error {
	if len(members) == 0 {
		return nil
	}

	for _, member := range members {
		dbMember := ToDBChatMember(chatID, member)
		if err := g.insertChatMember(ctx, chatID, dbMember); err != nil {
			return errors.Wrap(err, "failed to add chat member")
		}
	}
	return nil
}

func (g *ChatRepository) saveMessages(ctx context.Context, dbMessages []*models.Message) error {
	for _, m := range dbMessages {
		if m.ID == 0 {
			if _, err := g.insertMessage(ctx, m); err != nil {
				return errors.Wrap(err, "failed to add message")
			}
		} else {
			if err := g.updateMessage(ctx, m); err != nil {
				return errors.Wrap(err, "failed to update message")
			}
		}
	}
	return nil
}

func (g *ChatRepository) Save(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	if data.ID() == 0 {
		return g.create(ctx, data)
	}
	return g.update(ctx, data)
}

func (g *ChatRepository) create(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbChat, dbMessages := ToDBChat(data)
	if err := tx.QueryRow(
		ctx,
		insertChatQuery,
		dbChat.ClientID,
		&dbChat.CreatedAt,
	).Scan(&dbChat.ID); err != nil {
		return nil, errors.Wrap(err, "failed to insert chat")
	}

	// First save all members including those from messages
	var allMembers []chat.Member

	allMembers = append(allMembers, data.Members()...)

	for _, message := range data.Messages() {
		found := false
		for _, member := range allMembers {
			if member.ID() == message.Sender().ID() {
				found = true
				break
			}
		}

		if !found {
			allMembers = append(allMembers, message.Sender())
		}
	}

	// Save members first
	if err := g.saveMembers(ctx, dbChat.ID, allMembers); err != nil {
		return nil, errors.Wrap(err, "failed to save chat members")
	}

	// Set the chat ID for all messages
	for _, m := range dbMessages {
		m.ChatID = dbChat.ID
	}

	// Then save messages
	if err := g.saveMessages(ctx, dbMessages); err != nil {
		return nil, err
	}

	return g.GetByID(ctx, dbChat.ID)
}

func (g *ChatRepository) update(ctx context.Context, data chat.Chat) (chat.Chat, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get transaction")
	}

	dbChat, dbMessages := ToDBChat(data)
	if _, err := tx.Exec(
		ctx,
		updateChatQuery,
		dbChat.ClientID,
		&dbChat.CreatedAt,
		&dbChat.LastMessageAt,
		dbChat.ID,
	); err != nil {
		return nil, errors.Wrap(err, "failed to update chat")
	}

	// First save all members including those from messages
	var allMembers []chat.Member

	// Get members from the chat
	allMembers = append(allMembers, data.Members()...)

	// Also include members from messages
	for _, message := range data.Messages() {
		// Check if this member is already included
		found := false
		for _, member := range allMembers {
			if member.ID() == message.Sender().ID() {
				found = true
				break
			}
		}

		if !found {
			allMembers = append(allMembers, message.Sender())
		}
	}

	// Save members first
	if err := g.saveMembers(ctx, dbChat.ID, allMembers); err != nil {
		return nil, errors.Wrap(err, "failed to save chat members")
	}

	// Set the chat ID for all messages
	for _, m := range dbMessages {
		m.ChatID = dbChat.ID
	}

	// Then save messages
	if err := g.saveMessages(ctx, dbMessages); err != nil {
		return nil, err
	}

	return g.GetByID(ctx, dbChat.ID)
}

func (g *ChatRepository) Delete(ctx context.Context, id uint) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get transaction")
	}

	// First delete all messages for this chat
	_, err = tx.Exec(ctx, deleteMessageQuery, id)
	if err != nil {
		return errors.Wrapf(err, "failed to delete messages for chat with id %d", id)
	}

	// Then delete all members for this chat
	_, err = tx.Exec(ctx, deleteChatMembersQuery, id)
	if err != nil {
		return errors.Wrapf(err, "failed to delete members for chat with id %d", id)
	}

	// Finally delete the chat itself
	if _, err := tx.Exec(ctx, deleteChatQuery, id); err != nil {
		return errors.Wrapf(err, "failed to delete chat with id %d", id)
	}
	return nil
}
