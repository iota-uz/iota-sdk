// Package services provides this package.
package services

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig"
	"github.com/iota-uz/iota-sdk/pkg/security"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	pkgtwofactor "github.com/iota-uz/iota-sdk/pkg/twofactor"
)

type LoginAccessChecker func(ctx context.Context, u coreuser.User) error

type AuthenticationResult struct {
	User               coreuser.User
	Session            session.Session
	Method             pkgtwofactor.AuthMethod
	AuthenticatorID    string
	SatisfiesTwoFactor bool
}

type FinalizeAuthenticationOptions struct {
	NextURL     string
	AccessCheck LoginAccessChecker
}

type FinalizeAuthenticationResult struct {
	Cookie      *http.Cookie
	RedirectURL string
}

type UserVisibleError struct {
	Message string
	Err     error
}

func (e *UserVisibleError) Error() string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Message
}

func (e *UserVisibleError) Unwrap() error {
	return e.Err
}

type AuthFlowService struct {
	authService     *AuthService
	sessionService  *SessionService
	twoFactorPolicy pkgtwofactor.TwoFactorPolicy
	httpCfg         *httpconfig.Config
}

func NewAuthFlowService(authService *AuthService, sessionService *SessionService, httpCfg *httpconfig.Config) *AuthFlowService {
	return &AuthFlowService{
		authService:    authService,
		sessionService: sessionService,
		httpCfg:        httpCfg,
	}
}

func (s *AuthFlowService) SetTwoFactorPolicy(policy pkgtwofactor.TwoFactorPolicy) {
	s.twoFactorPolicy = policy
}

func (s *AuthFlowService) AuthenticatePassword(
	ctx context.Context,
	email string,
	password string,
) (*AuthenticationResult, error) {
	const op serrors.Op = "core.AuthFlowService.AuthenticatePassword"

	u, err := s.authService.VerifyPassword(ctx, email, password)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return &AuthenticationResult{
		User:            u,
		Method:          pkgtwofactor.AuthMethodPassword,
		AuthenticatorID: "password",
	}, nil
}

func (s *AuthFlowService) AuthenticateGoogle(ctx context.Context, code string) (*AuthenticationResult, error) {
	const op serrors.Op = "core.AuthFlowService.AuthenticateGoogle"

	u, err := s.authService.VerifyGoogle(ctx, code)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	return &AuthenticationResult{
		User:            u,
		Method:          pkgtwofactor.AuthMethodOAuth,
		AuthenticatorID: "google",
	}, nil
}

func (s *AuthFlowService) FinalizeAuthenticatedUser(
	ctx context.Context,
	u coreuser.User,
	method pkgtwofactor.AuthMethod,
	opts FinalizeAuthenticationOptions,
) (*FinalizeAuthenticationResult, error) {
	return s.FinalizeAuthentication(ctx, &AuthenticationResult{
		User:            u,
		Method:          method,
		AuthenticatorID: string(method),
	}, opts)
}

func (s *AuthFlowService) FinalizeAuthentication(
	ctx context.Context,
	auth *AuthenticationResult,
	opts FinalizeAuthenticationOptions,
) (*FinalizeAuthenticationResult, error) {
	const op serrors.Op = "core.AuthFlowService.FinalizeAuthentication"

	if auth == nil || auth.User == nil {
		return nil, serrors.E(op, serrors.Invalid, errors.New("authentication result is required"))
	}

	validatedNextURL := security.GetValidatedRedirect(opts.NextURL)

	if opts.AccessCheck != nil {
		if err := opts.AccessCheck(ctx, auth.User); err != nil {
			return nil, &UserVisibleError{
				Message: err.Error(),
				Err:     serrors.E(op, err),
			}
		}
	}

	sess := auth.Session
	if sess == nil {
		createdSession, err := s.authService.CreateSession(ctx, auth.User)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		sess = createdSession
	}

	requires2FA, err := s.requiresTwoFactor(ctx, auth)
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if requires2FA {
		pendingSession := session.New(
			sess.Token(),
			sess.UserID(),
			sess.TenantID(),
			sess.IP(),
			sess.UserAgent(),
			session.WithStatus(session.StatusPending2FA),
			session.WithAudience(sess.Audience()),
			session.WithExpiresAt(time.Now().Add(10*time.Minute)),
			session.WithCreatedAt(sess.CreatedAt()),
		)
		if err := s.sessionService.Update(ctx, pendingSession); err != nil {
			return nil, serrors.E(op, err)
		}

		redirectURL := fmt.Sprintf("/login/2fa/setup?next=%s", url.QueryEscape(validatedNextURL))
		if auth.User.Has2FAEnabled() {
			redirectURL = fmt.Sprintf("/login/2fa/verify?next=%s", url.QueryEscape(validatedNextURL))
		}

		return &FinalizeAuthenticationResult{
			Cookie:      s.sessionCookie(pendingSession.Token(), pendingSession.ExpiresAt()),
			RedirectURL: redirectURL,
		}, nil
	}

	return &FinalizeAuthenticationResult{
		Cookie:      s.sessionCookie(sess.Token(), sess.ExpiresAt()),
		RedirectURL: validatedNextURL,
	}, nil
}

func (s *AuthFlowService) requiresTwoFactor(
	ctx context.Context,
	auth *AuthenticationResult,
) (bool, error) {
	const op serrors.Op = "core.AuthFlowService.requiresTwoFactor"

	if auth.SatisfiesTwoFactor {
		return false, nil
	}

	requires2FA := auth.User.Has2FAEnabled()
	if s.twoFactorPolicy == nil {
		return requires2FA, nil
	}

	ip, _ := composables.UseIP(ctx)
	userAgent, _ := composables.UseUserAgent(ctx)

	attempt := pkgtwofactor.AuthAttempt{
		UserID:    userIDToNamespacedUUID(auth.User.TenantID(), auth.User.ID()),
		Method:    auth.Method,
		IPAddress: ip,
		UserAgent: userAgent,
		Timestamp: time.Now(),
	}
	policyRequires2FA, err := s.twoFactorPolicy.Requires(ctx, attempt)
	if err != nil {
		return false, serrors.E(op, err)
	}

	// Intentional OR: policies can tighten (add) the 2FA requirement but cannot relax it.
	// A user with 2FA already enabled always goes through 2FA regardless of policy result.
	return requires2FA || policyRequires2FA, nil
}

func (s *AuthFlowService) sessionCookie(token string, expiresAt time.Time) *http.Cookie {
	return &http.Cookie{
		Name:     s.httpCfg.Cookies.SID,
		Value:    token,
		Expires:  expiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.httpCfg.IsProduction(),
		Domain:   s.httpCfg.Domain,
		Path:     "/",
	}
}

func userIDToNamespacedUUID(tenantID uuid.UUID, userID uint) uuid.UUID {
	var userIDData [8]byte
	binary.LittleEndian.PutUint64(userIDData[:], uint64(userID))
	return uuid.NewSHA1(tenantID, userIDData[:])
}
