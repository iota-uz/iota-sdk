// Package share owns persisted Lens slices and scheduled workbook delivery.
package share

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

type ViewScope string

const (
	ViewScopePersonal ViewScope = "personal"
	ViewScopeTeam     ViewScope = "team"
)

var (
	ErrNotFound        = errors.New("lens share record not found")
	ErrForbidden       = errors.New("lens share operation forbidden")
	ErrClaimLost       = errors.New("lens export schedule claim lost")
	ErrInvalid         = errors.New("lens share input invalid")
	ErrUnauthenticated = errors.New("lens share authentication required")
)

func invalidInput(err error) error {
	return fmt.Errorf("%w: %s", ErrInvalid, err)
}

type Access struct {
	TenantID     uuid.UUID
	UserID       uint
	RoleIDs      []uint
	Roles        []RoleRef
	ManageTeam   bool
	ScheduleMail bool
}

type RoleRef struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

func canAssignDefaultRole(access Access, roleID uint) bool {
	if !access.ManageTeam || roleID == 0 {
		return false
	}
	for _, role := range access.Roles {
		if role.ID == roleID {
			return true
		}
	}
	return false
}

type SavedView struct {
	ID            uuid.UUID `json:"id"`
	TenantID      uuid.UUID `json:"-"`
	DashboardID   string    `json:"dashboardId"`
	Name          string    `json:"name"`
	Scope         ViewScope `json:"scope"`
	OwnerUserID   uint      `json:"ownerUserId"`
	StateURL      string    `json:"stateUrl"`
	DefaultRoleID *uint     `json:"defaultRoleId,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

func (v SavedView) Validate() error {
	if v.TenantID == uuid.Nil || v.OwnerUserID == 0 {
		return fmt.Errorf("tenant and owner are required")
	}
	if value := strings.TrimSpace(v.DashboardID); value == "" || len(value) > 120 {
		return fmt.Errorf("dashboardId must contain 1 to 120 characters")
	}
	if value := strings.TrimSpace(v.Name); value == "" || len([]rune(value)) > 120 {
		return fmt.Errorf("name must contain 1 to 120 characters")
	}
	if v.Scope != ViewScopePersonal && v.Scope != ViewScopeTeam {
		return fmt.Errorf("scope must be personal or team")
	}
	if v.DefaultRoleID != nil && (v.Scope != ViewScopeTeam || *v.DefaultRoleID == 0) {
		return fmt.Errorf("a role default must be a team view with a valid role")
	}
	parsed, err := url.Parse(v.StateURL)
	if err != nil || !strings.HasPrefix(v.StateURL, "/") || strings.HasPrefix(v.StateURL, "//") ||
		strings.Contains(v.StateURL, `\`) || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil {
		return fmt.Errorf("stateUrl must be a site-relative URL")
	}
	if len(v.StateURL) > 8*1024 {
		return fmt.Errorf("stateUrl cannot exceed 8192 characters")
	}
	return nil
}

type ExportSchedule struct {
	ID          uuid.UUID `json:"id"`
	TenantID    uuid.UUID `json:"-"`
	OwnerUserID uint      `json:"ownerUserId"`
	DashboardID string    `json:"dashboardId"`
	ViewID      uuid.UUID `json:"viewId"`
	StateURL    string    `json:"-"`
	Name        string    `json:"name"`
	Cron        string    `json:"cron"`
	Timezone    string    `json:"timezone"`
	Recipients  []string  `json:"recipients"`
	Enabled     bool      `json:"enabled"`
	NextRunAt   time.Time `json:"nextRunAt"`
	LastRunAt   time.Time `json:"lastRunAt,omitempty"`
	LastError   string    `json:"lastError,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func (s ExportSchedule) Validate() error {
	if s.TenantID == uuid.Nil || s.OwnerUserID == 0 || s.ViewID == uuid.Nil {
		return fmt.Errorf("tenant, owner, and saved view are required")
	}
	if value := strings.TrimSpace(s.DashboardID); value == "" || len(value) > 120 {
		return fmt.Errorf("dashboardId must contain 1 to 120 characters")
	}
	if value := strings.TrimSpace(s.Name); value == "" || len([]rune(value)) > 120 {
		return fmt.Errorf("name must contain 1 to 120 characters")
	}
	if len(s.Recipients) == 0 || len(s.Recipients) > 20 {
		return fmt.Errorf("one to twenty recipients are required")
	}
	for _, recipient := range s.Recipients {
		if _, err := validateMailbox(recipient); err != nil {
			return fmt.Errorf("invalid recipient %q", recipient)
		}
	}
	if _, err := scheduleParser().Parse(s.Cron); err != nil {
		return fmt.Errorf("invalid cron schedule: %w", err)
	}
	if _, err := time.LoadLocation(s.Timezone); err != nil {
		return fmt.Errorf("invalid timezone: %w", err)
	}
	return nil
}

func (s ExportSchedule) Next(after time.Time) (time.Time, error) {
	location, err := time.LoadLocation(s.Timezone)
	if err != nil {
		return time.Time{}, err
	}
	parsed, err := scheduleParser().Parse(s.Cron)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.Next(after.In(location)).UTC(), nil
}

func scheduleParser() cron.Parser {
	return cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
}
