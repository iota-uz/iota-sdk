package tgserver

import (
	"context"
	"database/sql"
	"errors"
	"sync"

	"github.com/gotd/td/session"
	"github.com/iota-agency/iota-erp/internal/domain/entities/telegram_session"
	"github.com/jmoiron/sqlx"
)

func NewDBSession(db *sqlx.DB, userID int) *DBSession {
	return &DBSession{
		userID: userID,
		db:     db,
		mux:    sync.RWMutex{},
	}
}

type DBSession struct {
	userID int
	db     *sqlx.DB
	mux    sync.RWMutex
}

// LoadSession loads session from memory.
func (s *DBSession) LoadSession(context.Context) ([]byte, error) {
	if s == nil {
		return nil, session.ErrNotFound
	}

	s.mux.RLock()
	defer s.mux.RUnlock()

	dest := &telegram_session.TelegramSession{} //nolint:exhaustruct
	if err := s.db.Get(dest, "SELECT session FROM telegram_sessions WHERE user_id = $1", s.userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, session.ErrNotFound
		}
		return nil, err
	}
	return dest.Session, nil
}

// StoreSession stores session to memory.
func (s *DBSession) StoreSession(_ context.Context, data []byte) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	_, err := s.db.Exec(
		"INSERT INTO telegram_sessions (user_id, session) VALUES ($1, $2) ON CONFLICT (user_id) DO UPDATE SET session = $2",
		s.userID,
		data,
	)
	return err
}
