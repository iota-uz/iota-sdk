package telegram

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A name that is already older than the limit when it is cached is not kept, and a younger one stays; the sweep at
// each insertion also drops entries that have grown old since.
func TestUsernameCacheDropsTheEntriesOfReplacedTokens(t *testing.T) {
	c := New(Config{})
	now := time.Now()
	oldBot, current := sha256.Sum256([]byte("1:old")), sha256.Sum256([]byte("2:new"))
	c.rememberName(oldBot, "old_bot", now.Add(-usernameTTL*6/5))
	_, kept := c.usernames.Load(oldBot)
	assert.False(t, kept, "an entry older than the limit is not kept")
	c.rememberName(current, "new_bot", now.Add(-usernameTTL/2))
	_, kept = c.usernames.Load(current)
	assert.True(t, kept, "a young entry stays")
	// A later insertion sweeps what has grown old meanwhile.
	c.usernames.Store(oldBot, cachedName{name: "old_bot", at: now.Add(-2 * usernameTTL)})
	c.rememberName(sha256.Sum256([]byte("3:other")), "other_bot", now)
	_, kept = c.usernames.Load(oldBot)
	assert.False(t, kept)
	_, kept = c.usernames.Load(current)
	assert.True(t, kept)
}

// An entry leaves when its time is up without anything being cached after it: the timer does it.
func TestUsernameCacheEntryLeavesByItself(t *testing.T) {
	c := New(Config{})
	fp := sha256.Sum256([]byte("1:old"))
	// An entry whose time is almost up: it is gone shortly, with no later insertion.
	c.rememberName(fp, "old_bot", time.Now().Add(-usernameTTL+50*time.Millisecond))
	_, kept := c.usernames.Load(fp)
	require.True(t, kept)
	require.Eventually(t, func() bool {
		_, ok := c.usernames.Load(fp)
		return !ok
	}, 5*time.Second, 10*time.Millisecond, "the entry was not dropped by its timer")
}
