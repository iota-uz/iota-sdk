package viewmodels

import (
	"crypto/sha256"
	"fmt"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/utils/useragent"
)

// Session represents a user session for presentation
type Session struct {
	TokenDisplay string
	Device       string
	Browser      string
	OS           string
	IPAddress    string
	CreatedAt    string
	IsCurrent    bool
	Icon         string
	FullToken    string // Hidden from display, used for revocation
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
		IPAddress:    s.IP(),
		CreatedAt:    s.CreatedAt().Format("2006-01-02 15:04:05"),
		IsCurrent:    s.Token() == currentToken,
		Icon:         deviceInfo.Icon,
		FullToken:    s.Token(), // Store raw token for revocation
	}
}

// truncateToken returns the first 8 characters of the token followed by "..."
// This is safe to display without exposing the full token
func truncateToken(token string) string {
	if len(token) <= 8 {
		return token
	}
	return token[:8] + "..."
}

// hashToken creates a SHA-256 hash of the token for safe storage/comparison
// We use this instead of the raw token to avoid exposing sensitive data
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", hash)
}

// AdminSessionViewModel represents a session with enriched user information
// for admin global sessions page
type AdminSessionViewModel struct {
	*Session        // Embed existing Session viewmodel
	User     *User  // User who owns this session
	RawToken string // Store raw token for admin operations (revoking sessions)
}

// NewAdminSessionViewModel creates an AdminSessionViewModel with user info
// Includes the raw token for admin operations like session revocation
func NewAdminSessionViewModel(sess session.Session, user *User, currentToken string) *AdminSessionViewModel {
	return &AdminSessionViewModel{
		Session:  SessionToViewModel(sess, currentToken),
		User:     user,
		RawToken: sess.Token(), // Store raw token for admin operations
	}
}
