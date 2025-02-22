package persistence

import (
	"database/sql"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/authlog"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func ToDomainUser(dbUser *models.User, dbUpload *models.Upload, roles []role.Role) (user.User, error) {
	var avatar upload.Upload
	if dbUpload != nil {
		avatar = ToDomainUpload(dbUpload)
	}
	return user.NewWithID(
		dbUser.ID,
		dbUser.FirstName,
		dbUser.LastName,
		dbUser.MiddleName.String,
		dbUser.Password.String,
		dbUser.Email,
		avatar,
		uint(dbUser.EmployeeID.Int32),
		dbUser.LastIP.String,
		user.UILanguage(dbUser.UILanguage),
		roles,
		dbUser.LastLogin.Time,
		dbUser.LastAction.Time,
		dbUser.CreatedAt,
		dbUser.UpdatedAt,
	), nil
}

func toDBUser(entity user.User) (*models.User, []*models.Role) {
	roles := make([]*models.Role, len(entity.Roles()))
	for i, r := range entity.Roles() {
		dbRole, _ := toDBRole(r)
		roles[i] = dbRole
	}
	return &models.User{
		ID:         entity.ID(),
		FirstName:  entity.FirstName(),
		LastName:   entity.LastName(),
		MiddleName: mapping.ValueToSQLNullString(entity.MiddleName()),
		Email:      entity.Email(),
		UILanguage: string(entity.UILanguage()),
		Password:   mapping.ValueToSQLNullString(entity.Password()),
		AvatarID:   mapping.ValueToSQLNullInt32(int32(entity.AvatarID())),
		EmployeeID: mapping.ValueToSQLNullInt32(int32(entity.EmployeeID())),
		LastIP:     mapping.ValueToSQLNullString(entity.LastIP()),
		LastLogin:  mapping.ValueToSQLNullTime(entity.LastLogin()),
		LastAction: mapping.ValueToSQLNullTime(entity.LastAction()),
		CreatedAt:  entity.CreatedAt(),
		UpdatedAt:  entity.UpdatedAt(),
	}, roles
}

func toDomainRole(dbRole *models.Role, permissions []*models.Permission) (role.Role, error) {
	domainPermissions := make([]*permission.Permission, len(permissions))
	for i, p := range permissions {
		dP, err := toDomainPermission(p)
		if err != nil {
			return nil, err
		}
		domainPermissions[i] = dP
	}
	return role.NewWithID(
		dbRole.ID,
		dbRole.Name,
		dbRole.Description.String,
		domainPermissions,
		dbRole.CreatedAt,
		dbRole.UpdatedAt,
	)
}

func toDBRole(entity role.Role) (*models.Role, []*models.Permission) {
	permissions := make([]*models.Permission, len(entity.Permissions()))
	for i, p := range entity.Permissions() {
		permissions[i] = toDBPermission(p)
	}
	return &models.Role{
		ID:          entity.ID(),
		Name:        entity.Name(),
		Description: mapping.ValueToSQLNullString(entity.Description()),
		CreatedAt:   entity.CreatedAt(),
		UpdatedAt:   entity.UpdatedAt(),
	}, permissions
}

func toDBPermission(entity *permission.Permission) *models.Permission {
	return &models.Permission{
		ID:       entity.ID.String(),
		Name:     entity.Name,
		Resource: string(entity.Resource),
		Action:   string(entity.Action),
		Modifier: string(entity.Modifier),
	}
}

func toDomainPermission(dbPermission *models.Permission) (*permission.Permission, error) {
	id, err := uuid.Parse(dbPermission.ID)
	if err != nil {
		return nil, err
	}
	return &permission.Permission{
		ID:       id,
		Name:     dbPermission.Name,
		Resource: permission.Resource(dbPermission.Resource),
		Action:   permission.Action(dbPermission.Action),
		Modifier: permission.Modifier(dbPermission.Modifier),
	}, nil
}

func ToDomainPin(s sql.NullString, c country.Country) (tax.Pin, error) {
	if !s.Valid {
		return tax.NilPin, nil
	}
	return tax.NewPin(s.String, c)
}

func ToDomainTin(s sql.NullString, c country.Country) (tax.Tin, error) {
	if !s.Valid {
		return tax.NilTin, nil
	}
	return tax.NewTin(s.String, c)
}

func ToDBUpload(upload upload.Upload) *models.Upload {
	return &models.Upload{
		ID:        upload.ID(),
		Path:      upload.Path(),
		Hash:      upload.Hash(),
		Size:      upload.Size().Bytes(),
		Mimetype:  upload.Mimetype().String(),
		CreatedAt: upload.CreatedAt(),
		UpdatedAt: upload.UpdatedAt(),
	}
}

func ToDomainUpload(dbUpload *models.Upload) upload.Upload {
	var mime *mimetype.MIME
	if dbUpload.Mimetype != "" {
		mime = mimetype.Lookup(dbUpload.Mimetype)
	}
	return upload.NewWithID(
		dbUpload.ID,
		dbUpload.Hash,
		dbUpload.Path,
		dbUpload.Size,
		mime,
		dbUpload.CreatedAt,
		dbUpload.UpdatedAt,
	)
}

func ToDBCurrency(entity *currency.Currency) *models.Currency {
	return &models.Currency{
		Code:      string(entity.Code),
		Name:      entity.Name,
		Symbol:    string(entity.Symbol),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func ToDomainCurrency(dbCurrency *models.Currency) (*currency.Currency, error) {
	code, err := currency.NewCode(dbCurrency.Code)
	if err != nil {
		return nil, err
	}
	symbol, err := currency.NewSymbol(dbCurrency.Symbol)
	if err != nil {
		return nil, err
	}
	return &currency.Currency{
		Code:   code,
		Name:   dbCurrency.Name,
		Symbol: symbol,
	}, nil
}

func ToDBTab(tab *tab.Tab) *models.Tab {
	return &models.Tab{
		ID:       tab.ID,
		Href:     tab.Href,
		Position: tab.Position,
		UserID:   tab.UserID,
	}
}

func ToDomainTab(dbTab *models.Tab) (*tab.Tab, error) {
	return &tab.Tab{
		ID:       dbTab.ID,
		Href:     dbTab.Href,
		Position: dbTab.Position,
		UserID:   dbTab.UserID,
	}, nil
}

func toDBSession(session *session.Session) *models.Session {
	return &models.Session{
		UserID:    session.UserID,
		Token:     session.Token,
		IP:        session.IP,
		UserAgent: session.UserAgent,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	}
}

func toDomainSession(dbSession *models.Session) *session.Session {
	return &session.Session{
		UserID:    dbSession.UserID,
		Token:     dbSession.Token,
		IP:        dbSession.IP,
		UserAgent: dbSession.UserAgent,
		CreatedAt: dbSession.CreatedAt,
		ExpiresAt: dbSession.ExpiresAt,
	}
}

func toDBAuthenticationLog(log *authlog.AuthenticationLog) *models.AuthenticationLog {
	return &models.AuthenticationLog{
		ID:        log.ID,
		UserID:    log.UserID,
		IP:        log.IP,
		UserAgent: log.UserAgent,
		CreatedAt: log.CreatedAt,
	}
}

func toDomainAuthenticationLog(dbLog *models.AuthenticationLog) *authlog.AuthenticationLog {
	return &authlog.AuthenticationLog{
		ID:        dbLog.ID,
		UserID:    dbLog.UserID,
		IP:        dbLog.IP,
		UserAgent: dbLog.UserAgent,
		CreatedAt: dbLog.CreatedAt,
	}
}
