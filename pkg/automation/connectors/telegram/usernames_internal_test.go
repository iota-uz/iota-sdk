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

// An entry leaves when its time is up without anything being cached after it: the timer does it. The timer is
// stood in for, so the test does not depend on how fast it runs.
func TestUsernameCacheEntryLeavesByItself(t *testing.T) {
	c := New(Config{})
	var scheduled []func()
	var delays []time.Duration
	c.afterFunc = func(d time.Duration, fn func()) { delays, scheduled = append(delays, d), append(scheduled, fn) }
	fp := sha256.Sum256([]byte("1:old"))
	c.rememberName(fp, "old_bot", time.Now())
	_, kept := c.usernames.Load(fp)
	require.True(t, kept)
	require.Len(t, scheduled, 1)
	assert.InDelta(t, float64(usernameTTL), float64(delays[0]), float64(time.Minute), "the timer is for the rest of the hour")
	scheduled[0]() // the hour is up, and nothing was cached since
	_, kept = c.usernames.Load(fp)
	assert.False(t, kept, "the entry is dropped without a later insertion")

	// A refreshed entry is not the one the earlier timer holds: it stays.
	c.rememberName(fp, "bot_one", time.Now())
	c.rememberName(fp, "bot_two", time.Now().Add(time.Second))
	require.Len(t, scheduled, 3)
	scheduled[1]()
	v, kept := c.usernames.Load(fp)
	require.True(t, kept, "the timer of the replaced entry leaves the new one")
	assert.Equal(t, "bot_two", v.(cachedName).name)
}
