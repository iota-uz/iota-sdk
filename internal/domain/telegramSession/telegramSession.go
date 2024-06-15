package telegramSession

import "time"

type TelegramSession struct {
	UserId    int       `db:"user_id"`
	Session   []byte    `db:"session"`
	CreatedAt time.Time `db:"created_at"`
}

func (t *TelegramSession) ToGraph() {
}
