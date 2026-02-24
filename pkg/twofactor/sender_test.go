package twofactor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStoreAndGetTestOTPCode_CleansUpExpiredEntries(t *testing.T) {
	t.Cleanup(func() {
		testOTPCache.mu.Lock()
		testOTPCache.codes = make(map[string]testOTPEntry)
		testOTPCache.mu.Unlock()
	})

	StoreTestOTPCode("expired@example.com", "123456", 5*time.Millisecond)
	time.Sleep(10 * time.Millisecond)
	StoreTestOTPCode("active@example.com", "654321", time.Minute)

	_, ok := GetTestOTPCode("expired@example.com")
	require.False(t, ok)

	keys := sortedOTPTestCacheKeys()
	require.Equal(t, []string{"active@example.com"}, keys)
}

func TestOTPCacheKeys_DeterministicAndNoPhoneVariantsForEmail(t *testing.T) {
	require.Equal(
		t,
		[]string{"user@example.com"},
		otpCacheKeys("user@example.com"),
	)

	require.Equal(
		t,
		[]string{"+1 (555) 123-0000", "1 (555) 123-0000", "15551230000", "+15551230000"},
		otpCacheKeys("+1 (555) 123-0000"),
	)
}
