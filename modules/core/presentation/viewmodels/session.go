// Package viewmodels provides this package.
package viewmodels

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/utils/useragent"
)

// Session represents a user session for presentation
type Session struct {
	TokenDisplay string
	Device       string
	Browser      string
	OS           string
	IPAddress    string
	CreatedAt    string // Display value; in the application time zone when one is configured
	CreatedAtISO string // RFC 3339 UTC timestamp for client-side or localized rendering
	IsCurrent    bool
	Icon         string
	TokenID      string // Hash of token for safe DOM ids and revocation routes
}

// SessionToViewModel converts a domain session entity to a presentation ViewModel
func SessionToViewModel(s session.Session, currentToken string) *Session {
	deviceInfo := useragent.Parse(s.UserAgent())

	// Build browser string
	browser := deviceInfo.Browser
	if deviceInfo.BrowserVersion != "" {
		browser = fmt.Sprintf("%s %s", deviceInfo.Browser, deviceInfo.BrowserVersion)
	}

	// Build OS string
	osStr := deviceInfo.OS
	if deviceInfo.OSVersion != "" {
		osStr = fmt.Sprintf("%s %s", deviceInfo.OS, deviceInfo.OSVersion)
	}

	return &Session{
		TokenDisplay: truncateToken(s.Token()),
		Device:       deviceInfo.Device,
		Browser:      browser,
		OS:           osStr,
		IPAddress:    hostOnly(s.IP()),
		CreatedAt:    s.CreatedAt().Format("2006-01-02 15:04:05"),
		CreatedAtISO: s.CreatedAt().UTC().Format(time.RFC3339),
		IsCurrent:    s.Token() == currentToken,
		Icon:         deviceInfo.Icon,
		TokenID:      hashToken(s.Token()),
	}
}

// SessionToViewModelContext is SessionToViewModel with the creation time
// rendered in the application time zone carried by ctx, when one is configured.
func SessionToViewModelContext(ctx context.Context, s session.Session, currentToken string) *Session {
	vm := SessionToViewModel(s, currentToken)
	if formatted, ok := composables.FormatDateTime(ctx, s.CreatedAt()); ok {
		vm.CreatedAt = formatted
	}
	return vm
}

// hostOnly drops the port from a "host:port" remote address; values without a
// port are returned unchanged.
func hostOnly(addr string) string {
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return host
	}
	return addr
}

// truncateToken returns at most the first 4 characters followed by "..." for display.
// Short tokens are masked as "****" to avoid exposing them.
func truncateToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:4] + "..."
}

// hashToken creates a SHA-256 hash of the token for safe storage/comparison
// We use this instead of the raw token to avoid exposing sensitive data
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}

// AdminSessionViewModel represents a session with enriched user information
// for admin global sessions page.
//
// SECURITY NOTE: This struct contains the RawToken which should only be
// exposed in admin-gated UI contexts (requires SessionDelete permission).
type AdminSessionViewModel struct {
	*Session        // Embed existing Session viewmodel
	User     *User  // User who owns this session
	RawToken string // Store raw token for admin operations (revoking sessions)
}

// NewAdminSessionViewModel creates an AdminSessionViewModel with user info
// Includes the raw token for admin operations like session revocation
func NewAdminSessionViewModel(sess session.Session, user *User, currentToken string) *AdminSessionViewModel {
	return NewAdminSessionViewModelContext(context.Background(), sess, user, currentToken)
}

// NewAdminSessionViewModelContext is NewAdminSessionViewModel with the creation
// time rendered in the application time zone carried by ctx.
func NewAdminSessionViewModelContext(ctx context.Context, sess session.Session, user *User, currentToken string) *AdminSessionViewModel {
	return &AdminSessionViewModel{
		Session:  SessionToViewModelContext(ctx, sess, currentToken),
		User:     user,
		RawToken: sess.Token(), // Store raw token for admin operations
	}
}
