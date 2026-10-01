package controllers

import (
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/url"

	"github.com/gabriel-vasile/mimetype"
	"github.com/gorilla/mux"

	coreuser "github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/pages/onboarding"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/iota-uz/iota-sdk/pkg/validators"
)

const (
	onboardingMaxFormMemory = 10 << 20
	onboardingMaxBodyBytes  = 12 << 20
)

// OnboardingController serves the only page a pending-onboarding session can
// open: the user chooses a language, fills in the profile and sets their own
// password.
type OnboardingController struct {
	app             application.Application
	userService     *services.UserService
	uploadService   *services.UploadService
	browserSessions *services.BrowserSessionService
}

func NewOnboardingController(
	app application.Application,
	userService *services.UserService,
	uploadService *services.UploadService,
	browserSessions *services.BrowserSessionService,
) application.Controller {
	return &OnboardingController{
		app:             app,
		userService:     userService,
		uploadService:   uploadService,
		browserSessions: browserSessions,
	}
}

func (c *OnboardingController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("core.onboarding", 0, application.Route("", services.OnboardingPath))
}

func (c *OnboardingController) Register(r *mux.Router) {
	router := r.PathPrefix(services.OnboardingPath).Subrouter()
	router.Use(
		middleware.AuthorizeAnySession(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", c.Get).Methods(http.MethodGet)
	router.HandleFunc("", c.Post).Methods(http.MethodPost)
}

// pendingUser returns the user behind a live onboarding session, or redirects
// the request away from onboarding.
func (c *OnboardingController) pendingUser(w http.ResponseWriter, r *http.Request) (session.Session, coreuser.User, bool) {
	sess, err := composables.UseSession(r.Context())
	if err != nil || sess.IsExpired() {
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil, nil, false
	}
	if !sess.IsPendingOnboarding() {
		http.Redirect(w, r, "/", http.StatusFound)
		return nil, nil, false
	}
	u, err := c.userService.GetByID(r.Context(), sess.UserID())
	if err != nil {
		composables.UseLogger(r.Context()).WithError(err).Error("onboarding: failed to load user")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return nil, nil, false
	}
	if u.IsBlocked() || !u.IsPendingOnboarding() {
		c.endOnboardingSession(w, r)
		messageID := "Login.Errors.SessionExpired"
		if u.IsBlocked() {
			messageID = "Login.Errors.AccountBlocked"
		}
		shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), messageID)))
		http.Redirect(w, r, "/login", http.StatusFound)
		return nil, nil, false
	}
	return sess, u, true
}

func (c *OnboardingController) endOnboardingSession(w http.ResponseWriter, r *http.Request) {
	if c.browserSessions == nil {
		return
	}
	if _, err := c.browserSessions.RemoveCurrent(w, r); err != nil {
		composables.UseLogger(r.Context()).WithError(err).Warn("onboarding: failed to clear browser session")
	}
}

func (c *OnboardingController) render(w http.ResponseWriter, r *http.Request, u coreuser.User, dto *dtos.OnboardingDTO, errs map[string]string) {
	if dto == nil {
		dto = &dtos.OnboardingDTO{
			FirstName:  u.FirstName(),
			LastName:   u.LastName(),
			MiddleName: u.MiddleName(),
			Language:   string(u.UILanguage()),
		}
		if u.Phone() != nil {
			dto.Phone = u.Phone().Value()
		}
	}
	props := &onboarding.Props{
		Email:           u.Email().Value(),
		FirstName:       dto.FirstName,
		LastName:        dto.LastName,
		MiddleName:      dto.MiddleName,
		Phone:           dto.Phone,
		Language:        dto.Language,
		Languages:       intl.GetSupportedLanguages(c.app.GetSupportedLanguages()),
		MinPasswordSize: coreuser.MinPasswordLength,
		Errors:          errs,
	}
	if len(errs) > 0 {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	if err := onboarding.Index(props).Render(r.Context(), w); err != nil {
		composables.UseLogger(r.Context()).WithError(err).Error("onboarding: failed to render page")
	}
}

func (c *OnboardingController) Get(w http.ResponseWriter, r *http.Request) {
	_, u, ok := c.pendingUser(w, r)
	if !ok {
		return
	}
	c.render(w, r, u, nil, nil)
}

func (c *OnboardingController) Post(w http.ResponseWriter, r *http.Request) {
	sess, u, ok := c.pendingUser(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, onboardingMaxBodyBytes)
	if err := r.ParseMultipartForm(onboardingMaxFormMemory); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "Request Entity Too Large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	dto, err := composables.UseForm(&dtos.OnboardingDTO{}, r)
	if err != nil {
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}
	errs, valid := dto.Ok(r.Context(), c.app.GetSupportedLanguages())
	if err := validateAvatar(r); err != nil {
		errs["Avatar"] = intl.MustT(r.Context(), "Onboarding.Errors.AvatarNotImage")
		valid = false
	}
	if _, reported := errs["NewPassword"]; !reported && u.CheckPassword(dto.NewPassword) {
		errs["NewPassword"] = intl.MustT(r.Context(), "Onboarding.Errors.PasswordReusesTemporary")
		valid = false
	}
	if !valid {
		c.render(w, r, u, dto, errs)
		return
	}

	var completed coreuser.User
	err = composables.InTx(r.Context(), func(txCtx context.Context) error {
		avatarID, err := c.saveAvatar(txCtx, r)
		if err != nil {
			return err
		}
		input, err := dto.ToInput(avatarID)
		if err != nil {
			return err
		}
		completed, err = c.userService.CompleteOnboarding(txCtx, sess.Token(), input)
		return err
	})
	if err != nil {
		if errs, handled := c.onboardingErrors(r.Context(), err); handled {
			c.render(w, r, u, dto, errs)
			return
		}
		if messageID, ok := onboardingEndedMessageID(err); ok {
			c.endOnboardingSession(w, r)
			shared.SetFlash(w, "error", []byte(intl.MustT(r.Context(), messageID)))
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		composables.UseLogger(r.Context()).WithError(err).Error("onboarding: failed to complete")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	c.endOnboardingSession(w, r)
	shared.SetFlash(w, "notice", []byte(intl.MustT(r.Context(), "Onboarding.Completed")))
	http.Redirect(w, r, "/login?"+url.Values{"email": []string{completed.Email().Value()}}.Encode(), http.StatusFound)
}

// onboardingEndedMessageID maps errors that end the onboarding session.
func onboardingEndedMessageID(err error) (string, bool) {
	switch {
	case errors.Is(err, services.ErrUserBlocked):
		return "Login.Errors.AccountBlocked", true
	case errors.Is(err, coreuser.ErrTemporaryPasswordExpired), errors.Is(err, coreuser.ErrTemporaryPasswordExhausted):
		return "Login.Errors.TemporaryPasswordExpired", true
	case errors.Is(err, services.ErrOnboardingSessionInvalid), errors.Is(err, coreuser.ErrNotPendingOnboarding):
		return "Login.Errors.SessionExpired", true
	}
	return "", false
}

var errAvatarNotImage = errors.New("avatar must be a raster image")

// avatarMimeTypes excludes SVG: uploads are served publicly and SVG can carry script.
var avatarMimeTypes = []string{"image/png", "image/jpeg", "image/gif", "image/webp"}

func avatarFile(r *http.Request) *multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}
	files := r.MultipartForm.File["Avatar"]
	if len(files) == 0 || files[0].Size == 0 {
		return nil
	}
	return files[0]
}

// validateAvatar sniffs the content before anything is written to storage.
func validateAvatar(r *http.Request) error {
	header := avatarFile(r)
	if header == nil {
		return nil
	}
	file, err := header.Open()
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()
	detected, err := mimetype.DetectReader(file)
	if err != nil {
		return err
	}
	for _, allowed := range avatarMimeTypes {
		if detected.Is(allowed) {
			return nil
		}
	}
	return errAvatarNotImage
}

func (c *OnboardingController) saveAvatar(ctx context.Context, r *http.Request) (uint, error) {
	header := avatarFile(r)
	if header == nil {
		return 0, nil
	}
	file, err := header.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = file.Close() }()
	created, err := c.uploadService.Create(ctx, &upload.CreateDTO{
		File: file,
		Name: header.Filename,
		Size: int(header.Size),
	})
	if err != nil {
		return 0, err
	}
	return created.ID(), nil
}

func (c *OnboardingController) onboardingErrors(ctx context.Context, err error) (map[string]string, bool) {
	var validationErr *validators.ValidationError
	if errors.As(err, &validationErr) {
		return validationErr.Fields, true
	}
	if messageID := dtos.PasswordPolicyMessageID(err); messageID != "Onboarding.Errors.PasswordInvalid" && messageID != "" {
		return map[string]string{"NewPassword": intl.MustT(ctx, messageID)}, true
	}
	return nil, false
}
