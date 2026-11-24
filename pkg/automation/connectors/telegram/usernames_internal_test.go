package telegram

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// A bot whose token was replaced is never asked for again: its cached name goes when the next one is cached,
// once it is older than usernameTTL, and the current bot's entry stays.
func TestUsernameCacheDropsTheEntriesOfReplacedTokens(t *testing.T) {
	c := New(Config{})
	now := time.Now()
	oldBot, current := sha256.Sum256([]byte("1:old")), sha256.Sum256([]byte("2:new"))
	c.rememberName(oldBot, "old_bot", now.Add(-usernameTTL*6/5))
	c.rememberName(current, "new_bot", now.Add(-usernameTTL/2))
	_, kept := c.usernames.Load(oldBot)
	assert.True(t, kept, "the entry is young when the next one is made")
	c.rememberName(sha256.Sum256([]byte("3:other")), "other_bot", now)
	_, kept = c.usernames.Load(oldBot)
	assert.False(t, kept, "an entry older than the limit is dropped")
	_, kept = c.usernames.Load(current)
	assert.True(t, kept, "a young entry stays")
}
