package persistence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/website/domain/entities/chatthread"
	"github.com/iota-uz/iota-sdk/modules/website/infrastructure/persistence/models"
	"github.com/redis/go-redis/v9"
)

type ThreadRepository struct {
	redis  *redis.Client
	prefix string
}

func NewThreadRepository(redis *redis.Client) *ThreadRepository {
	return &ThreadRepository{redis: redis, prefix: "threads"}
}

func (r *ThreadRepository) GetByID(ctx context.Context, id uuid.UUID) (chatthread.ChatThread, error) {
	var model models.ChatThread
	result, err := r.redis.Get(ctx, r.getKey(id.String())).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, chatthread.ErrChatThreadNotFound
		}
		return nil, err
	}
	if err := json.Unmarshal([]byte(result), &model); err != nil {
		return nil, err
	}

	return ToDomainChatThread(model)
}

func (r *ThreadRepository) Save(ctx context.Context, thread chatthread.ChatThread) (chatthread.ChatThread, error) {
	threadJson, err := json.Marshal(ToDBChatThread(thread))
	if err != nil {
		return nil, err
	}
	if err := r.redis.Set(ctx, r.getKey(thread.ID().String()), threadJson, 0).Err(); err != nil {
		return nil, err
	}

	return thread, nil
}

func (r *ThreadRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.redis.Del(ctx, r.getKey(id.String())).Err()
}

func (r *ThreadRepository) List(ctx context.Context) ([]chatthread.ChatThread, error) {
	var cursor uint64

	threads := make([]chatthread.ChatThread, 0)

	for {
		keys, nextCursor, err := r.redis.Scan(ctx, cursor, r.prefix+"*", 100).Result()
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			result, err := r.redis.Get(ctx, key).Result()
			if err != nil {
				return nil, err
			}

			var model models.ChatThread
			if err := json.Unmarshal([]byte(result), &model); err != nil {
				return nil, err
			}
			thread, err := ToDomainChatThread(model)
			if err != nil {
				return nil, err
			}
			threads = append(threads, thread)
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return threads, nil
}

func (r *ThreadRepository) getKey(key string) string {
	return fmt.Sprintf("%s:%s", r.prefix, key)
}
