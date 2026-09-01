package services

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/cookies"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

const (
	browserSessionCookieVersion = 1
	MaxBrowserSessions          = 5
)

type browserSessionEntry struct {
	Token      string `json:"token"`
	LastActive int64  `json:"last_active"`
}

type browserSessionState struct {
	Version int                   `json:"version"`
	Active  string                `json:"active"`
	Entries []browserSessionEntry `json:"entries"`
}

type BrowserSession struct {
	Session session.Session
	User    coreuser.User
	Active  bool
}

func (s BrowserSession) Reference() string {
	return BrowserSessionReference(s.Session.Token())
}

type browserSessionsContextKey struct{}

func WithBrowserSessions(ctx context.Context, sessions []BrowserSession) context.Context {
	return context.WithValue(ctx, browserSessionsContextKey{}, sessions)
}

func UseBrowserSessions(ctx context.Context) []BrowserSession {
	sessions, _ := ctx.Value(browserSessionsContextKey{}).([]BrowserSession)
	return sessions
}

func BrowserSessionReference(token string) string {
	digest := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(digest[:18])
}

type BrowserSessionService struct {
	sessionService        *SessionService
	userRepo              coreuser.Repository
	cookiesCfg            *cookies.Config
	appCfg                *appconfig.Config
	now                   func() time.Time
	authorizationRequests AuthorizationRequestValidator
}

type AuthorizationRequestValidator interface {
	ValidateAuthorizationRequest(ctx context.Context, id string) error
}

func (s *BrowserSessionService) SetAuthorizationRequestValidator(validator AuthorizationRequestValidator) {
	s.authorizationRequests = validator
}

func (s *BrowserSessionService) ValidateAuthorizationRequest(ctx context.Context, id string) error {
	const op serrors.Op = "core.BrowserSessionService.ValidateAuthorizationRequest"

	if strings.TrimSpace(id) == "" {
		return nil
	}
	if s.authorizationRequests == nil {
		return serrors.E(op, errors.New("authorization requests are unavailable"))
	}
	if err := s.authorizationRequests.ValidateAuthorizationRequest(ctx, id); err != nil {
		return serrors.E(op, err)
	}
	return nil
}

func NewBrowserSessionService(
	sessionService *SessionService,
	userRepo coreuser.Repository,
	cookiesCfg *cookies.Config,
	appCfg *appconfig.Config,
) *BrowserSessionService {
	return &BrowserSessionService{
		sessionService: sessionService,
		userRepo:       userRepo,
		cookiesCfg:     cookiesCfg,
		appCfg:         appCfg,
		now:            time.Now,
	}
}

func (s *BrowserSessionService) Resolve(w http.ResponseWriter, r *http.Request) ([]BrowserSession, error) {
	const op serrors.Op = "core.BrowserSessionService.Resolve"

	value := ""
	if cookie, err := r.Cookie(s.sidName()); err == nil {
		value = cookie.Value
	} else if !errors.Is(err, http.ErrNoCookie) {
		return nil, serrors.E(op, err)
	}

	state, sessions, changed, err := s.resolveValue(r.Context(), value)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	if changed && w != nil {
		http.SetCookie(w, s.cookie(state, sessions))
	}
	return sessions, nil
}

func (s *BrowserSessionService) Active(w http.ResponseWriter, r *http.Request) (BrowserSession, error) {
	const op serrors.Op = "core.BrowserSessionService.Active"

	sessions, err := s.Resolve(w, r)
	if err != nil {
		return BrowserSession{}, serrors.E(op, err)
	}
	for _, browserSession := range sessions {
		if browserSession.Active {
			return browserSession, nil
		}
	}
	return BrowserSession{}, serrors.E(op, persistence.ErrSessionNotFound)
}

func (s *BrowserSessionService) Add(ctx context.Context, cookieValue string, sess session.Session) (*http.Cookie, error) {
	const op serrors.Op = "core.BrowserSessionService.Add"

	state, sessions, _, err := s.resolveValue(ctx, cookieValue)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	now := s.now().UnixNano()
	entries := make([]browserSessionEntry, 0, len(state.Entries)+1)
	entries = append(entries, browserSessionEntry{Token: sess.Token(), LastActive: now})
	for _, entry := range state.Entries {
		if entry.Token != sess.Token() {
			entries = append(entries, entry)
		}
	}
	state.Active = sess.Token()
	state.Entries = entries
	sessions = append([]BrowserSession{{Session: sess, Active: true}}, withoutToken(sessions, sess.Token())...)
	for i := 1; i < len(sessions); i++ {
		sessions[i].Active = false
	}

	if len(state.Entries) > MaxBrowserSessions {
		evicted := state.Entries[MaxBrowserSessions:]
		state.Entries = state.Entries[:MaxBrowserSessions]
		sessions = sessions[:MaxBrowserSessions]
		for _, entry := range evicted {
			if err := s.deleteToken(ctx, entry.Token); err != nil && !errors.Is(err, persistence.ErrSessionNotFound) {
				return nil, serrors.E(op, err)
			}
		}
	}

	return s.cookie(state, sessions), nil
}

func (s *BrowserSessionService) AddFromRequest(ctx context.Context, r *http.Request, sess session.Session) (*http.Cookie, error) {
	value := ""
	if cookie, err := r.Cookie(s.sidName()); err == nil {
		value = cookie.Value
	}
	return s.Add(ctx, value, sess)
}

func (s *BrowserSessionService) Activate(w http.ResponseWriter, r *http.Request, reference string) (BrowserSession, error) {
	const op serrors.Op = "core.BrowserSessionService.Activate"

	state, sessions, changed, err := s.resolveRequest(r)
	if err != nil {
		return BrowserSession{}, serrors.E(op, err)
	}
	for i, browserSession := range sessions {
		if browserSession.Reference() != reference || !browserSession.Session.IsActive() {
			continue
		}
		state.Active = browserSession.Session.Token()
		for j := range state.Entries {
			if state.Entries[j].Token == state.Active {
				state.Entries[j].LastActive = s.now().UnixNano()
			}
		}
		sortEntries(state.Entries)
		for j := range sessions {
			sessions[j].Active = j == i
		}
		http.SetCookie(w, s.cookie(state, sessions))
		return browserSession, nil
	}
	if changed {
		http.SetCookie(w, s.cookie(state, sessions))
	}
	return BrowserSession{}, serrors.E(op, persistence.ErrSessionNotFound)
}

func (s *BrowserSessionService) RemoveCurrent(w http.ResponseWriter, r *http.Request) ([]BrowserSession, error) {
	const op serrors.Op = "core.BrowserSessionService.RemoveCurrent"

	state, sessions, _, err := s.resolveRequest(r)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	activeToken := state.Active
	if activeToken != "" {
		if err := s.deleteToken(r.Context(), activeToken); err != nil && !errors.Is(err, persistence.ErrSessionNotFound) {
			return nil, serrors.E(op, err)
		}
	}
	state.Entries = withoutEntry(state.Entries, activeToken)
	sessions = withoutToken(sessions, activeToken)
	state.Active = ""
	if len(state.Entries) > 0 {
		state.Active = state.Entries[0].Token
	}
	for i := range sessions {
		sessions[i].Active = sessions[i].Session.Token() == state.Active
	}
	http.SetCookie(w, s.cookie(state, sessions))
	return sessions, nil
}

func (s *BrowserSessionService) RemoveAll(w http.ResponseWriter, r *http.Request) error {
	const op serrors.Op = "core.BrowserSessionService.RemoveAll"

	state, _, _, err := s.resolveRequest(r)
	if err != nil {
		return serrors.E(op, err)
	}
	for _, entry := range state.Entries {
		if err := s.deleteToken(r.Context(), entry.Token); err != nil && !errors.Is(err, persistence.ErrSessionNotFound) {
			return serrors.E(op, err)
		}
	}
	http.SetCookie(w, s.clearCookie())
	return nil
}

func (s *BrowserSessionService) Clear(w http.ResponseWriter) {
	http.SetCookie(w, s.clearCookie())
}

func (s *BrowserSessionService) resolveRequest(r *http.Request) (browserSessionState, []BrowserSession, bool, error) {
	value := ""
	if cookie, err := r.Cookie(s.sidName()); err == nil {
		value = cookie.Value
	}
	return s.resolveValue(r.Context(), value)
}

func (s *BrowserSessionService) resolveValue(ctx context.Context, value string) (browserSessionState, []BrowserSession, bool, error) {
	state, migrated := decodeBrowserSessionState(value, s.now())
	changed := migrated
	resolved := make([]BrowserSession, 0, len(state.Entries))
	validEntries := make([]browserSessionEntry, 0, len(state.Entries))
	for _, entry := range state.Entries {
		sess, err := s.sessionService.GetBrowserSessionByToken(ctx, entry.Token)
		if errors.Is(err, persistence.ErrSessionNotFound) {
			changed = true
			continue
		}
		if err != nil {
			return state, nil, changed, serrors.E("core.BrowserSessionService.resolveValue", err)
		}
		if sess.Audience() != "" || sess.IsExpired() || (!sess.IsActive() && !sess.IsPending()) {
			changed = true
			continue
		}
		userCtx := composables.WithTenantID(ctx, sess.TenantID())
		u, err := s.userRepo.GetByID(userCtx, sess.UserID())
		if errors.Is(err, persistence.ErrUserNotFound) {
			changed = true
			continue
		}
		if err != nil {
			return state, nil, changed, serrors.E("core.BrowserSessionService.resolveValue", err)
		}
		if u.IsBlocked() {
			changed = true
			continue
		}
		validEntries = append(validEntries, entry)
		resolved = append(resolved, BrowserSession{Session: sess, User: u, Active: entry.Token == state.Active})
	}
	state.Entries = validEntries
	if !containsToken(state.Entries, state.Active) {
		state.Active = ""
		if len(state.Entries) > 0 {
			state.Active = state.Entries[0].Token
		}
		changed = true
	}
	for i := range resolved {
		resolved[i].Active = resolved[i].Session.Token() == state.Active
	}
	return state, resolved, changed, nil
}

func (s *BrowserSessionService) deleteToken(ctx context.Context, token string) error {
	sess, err := s.sessionService.GetBrowserSessionByToken(ctx, token)
	if err != nil {
		return err
	}
	return s.sessionService.Delete(composables.WithTenantID(ctx, sess.TenantID()), token)
}

func (s *BrowserSessionService) cookie(state browserSessionState, sessions []BrowserSession) *http.Cookie {
	if len(state.Entries) == 0 {
		return s.clearCookie()
	}
	expires := s.now()
	for _, browserSession := range sessions {
		if browserSession.Session.ExpiresAt().After(expires) {
			expires = browserSession.Session.ExpiresAt()
		}
	}
	encoded, err := encodeBrowserSessionState(state)
	if err != nil {
		return s.clearCookie()
	}
	return &http.Cookie{
		Name:     s.sidName(),
		Value:    encoded,
		Expires:  expires,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.appCfg.IsProduction(),
		Domain:   s.cookiesCfg.Domain,
		Path:     "/",
	}
}

func (s *BrowserSessionService) clearCookie() *http.Cookie {
	return &http.Cookie{
		Name:     s.sidName(),
		Value:    "",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.appCfg.IsProduction(),
		Domain:   s.cookiesCfg.Domain,
		Path:     "/",
	}
}

func (s *BrowserSessionService) sidName() string {
	if s.cookiesCfg != nil && s.cookiesCfg.SID != "" {
		return s.cookiesCfg.SID
	}
	return "sid"
}

func encodeBrowserSessionState(state browserSessionState) (string, error) {
	state.Version = browserSessionCookieVersion
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	return "v1." + base64.RawURLEncoding.EncodeToString(payload), nil
}

func decodeBrowserSessionState(value string, now time.Time) (browserSessionState, bool) {
	empty := browserSessionState{Version: browserSessionCookieVersion, Entries: []browserSessionEntry{}}
	value = strings.TrimSpace(value)
	if value == "" {
		return empty, false
	}
	if !strings.HasPrefix(value, "v1.") {
		empty.Active = value
		empty.Entries = []browserSessionEntry{{Token: value, LastActive: now.UnixNano()}}
		return empty, true
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, "v1."))
	if err != nil {
		return empty, true
	}
	var state browserSessionState
	if err := json.Unmarshal(payload, &state); err != nil || state.Version != browserSessionCookieVersion {
		return empty, true
	}
	seen := make(map[string]struct{}, len(state.Entries))
	entries := make([]browserSessionEntry, 0, len(state.Entries))
	for _, entry := range state.Entries {
		entry.Token = strings.TrimSpace(entry.Token)
		if entry.Token == "" {
			continue
		}
		if _, ok := seen[entry.Token]; ok {
			continue
		}
		seen[entry.Token] = struct{}{}
		entries = append(entries, entry)
	}
	state.Entries = entries
	sortEntries(state.Entries)
	if len(state.Entries) > MaxBrowserSessions {
		state.Entries = state.Entries[:MaxBrowserSessions]
		return state, true
	}
	return state, false
}

func sortEntries(entries []browserSessionEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].LastActive > entries[j].LastActive })
}

func containsToken(entries []browserSessionEntry, token string) bool {
	for _, entry := range entries {
		if entry.Token == token {
			return true
		}
	}
	return false
}

func withoutEntry(entries []browserSessionEntry, token string) []browserSessionEntry {
	filtered := entries[:0]
	for _, entry := range entries {
		if entry.Token != token {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func withoutToken(sessions []BrowserSession, token string) []BrowserSession {
	filtered := make([]BrowserSession, 0, len(sessions))
	for _, browserSession := range sessions {
		if browserSession.Session.Token() != token {
			filtered = append(filtered, browserSession)
		}
	}
	return filtered
}
