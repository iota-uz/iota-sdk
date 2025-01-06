package persistence

import (
	"database/sql"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/authlog"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/email"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/position"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/money"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func ToDomainUser(dbUser *models.User) (*user.User, error) {
	roles := make([]role.Role, len(dbUser.Roles))
	// for i, r := range dbUser.Roles {
	// 	roles[i] = toDomainRole(&r)
	// }
	var middleName string
	if dbUser.MiddleName != nil {
		middleName = *dbUser.MiddleName
	}
	var password string
	if dbUser.Password != nil {
		password = *dbUser.Password
	}
	var avatar upload.Upload
	if dbUser.Avatar != nil {
		avatar = *ToDomainUpload(dbUser.Avatar)
	}

	return &user.User{
		ID:         dbUser.ID,
		FirstName:  dbUser.FirstName,
		LastName:   dbUser.LastName,
		MiddleName: middleName,
		Email:      dbUser.Email,
		Password:   password,
		AvatarID:   dbUser.AvatarID,
		Avatar:     &avatar,
		EmployeeID: dbUser.EmployeeID,
		UILanguage: user.UILanguage(dbUser.UILanguage),
		LastIP:     dbUser.LastIP,
		LastLogin:  dbUser.LastLogin,
		LastAction: dbUser.LastAction,
		CreatedAt:  dbUser.CreatedAt,
		UpdatedAt:  dbUser.UpdatedAt,
		Roles:      roles,
	}, nil
}

func toDBUser(entity *user.User) (*models.User, []models.Role) {
	roles := make([]models.Role, len(entity.Roles))
	for i, r := range entity.Roles {
		dbRole, _ := toDBRole(r)
		roles[i] = *dbRole
	}
	var avatar *models.Upload
	if v := entity.AvatarID; v != nil {
		avatar = ToDBUpload(&upload.Upload{
			ID: *v,
		})
	}
	var avatarID *uint
	if entity.AvatarID != nil && *entity.AvatarID != 0 {
		avatarID = entity.AvatarID
	}
	return &models.User{
		ID:         entity.ID,
		FirstName:  entity.FirstName,
		LastName:   entity.LastName,
		MiddleName: &entity.MiddleName,
		Email:      entity.Email,
		UILanguage: string(entity.UILanguage),
		Password:   &entity.Password,
		AvatarID:   avatarID,
		EmployeeID: entity.EmployeeID,
		Avatar:     avatar,
		LastIP:     entity.LastIP,
		LastLogin:  entity.LastLogin,
		LastAction: entity.LastAction,
		CreatedAt:  entity.CreatedAt,
		UpdatedAt:  entity.UpdatedAt,
		Roles:      nil,
	}, roles
}

func toDomainRole(dbRole *models.Role, permissions []*models.Permission) (role.Role, error) {
	domainPermissions := make([]permission.Permission, len(permissions))
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

func toDBPermission(entity permission.Permission) *models.Permission {
	return &models.Permission{
		ID:       entity.ID,
		Name:     entity.Name,
		Resource: string(entity.Resource),
		Action:   string(entity.Action),
		Modifier: string(entity.Modifier),
	}
}

func toDomainPermission(dbPermission *models.Permission) (permission.Permission, error) {
	return permission.Permission{
		ID:       dbPermission.ID,
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

func toDomainEmployee(dbEmployee *models.Employee, dbMeta *models.EmployeeMeta) (employee.Employee, error) {
	tin, err := ToDomainTin(dbMeta.Tin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	pin, err := ToDomainPin(dbMeta.Pin, country.Uzbekistan)
	if err != nil {
		return nil, err
	}
	mail, err := email.New(dbEmployee.Email)
	if err != nil {
		return nil, err
	}
	var avatarID uint
	if dbEmployee.AvatarID != nil {
		avatarID = *dbEmployee.AvatarID
	}
	currencyCode, err := currency.NewCode(dbEmployee.SalaryCurrencyID)
	if err != nil {
		return nil, err
	}
	return employee.NewWithID(
		dbEmployee.ID,
		dbEmployee.FirstName,
		dbEmployee.LastName,
		dbEmployee.MiddleName.String,
		dbEmployee.Phone.String,
		mail,
		money.New(dbEmployee.Salary, currencyCode),
		tin,
		pin,
		employee.NewLanguage(dbMeta.PrimaryLanguage.String, dbMeta.SecondaryLanguage.String),
		dbMeta.HireDate.Time,
		mapping.SQLNullTimeToPointer(dbMeta.ResignationDate),
		avatarID,
		dbMeta.Notes.String,
		dbEmployee.CreatedAt,
		dbEmployee.UpdatedAt,
	), nil
}

func toDBEmployee(entity employee.Employee) (*models.Employee, *models.EmployeeMeta) {
	salary := entity.Salary()
	dbEmployee := &models.Employee{
		ID:               entity.ID(),
		FirstName:        entity.FirstName(),
		LastName:         entity.LastName(),
		MiddleName:       mapping.ValueToSQLNullString(entity.MiddleName()),
		Salary:           salary.Value(),
		SalaryCurrencyID: string(salary.Currency()),
		Email:            entity.Email().Value(),
		Phone:            mapping.ValueToSQLNullString(entity.Phone()),
		CreatedAt:        entity.CreatedAt(),
		UpdatedAt:        entity.UpdatedAt(),
	}
	lang := entity.Language()
	dbEmployeeMeta := &models.EmployeeMeta{
		PrimaryLanguage:   mapping.ValueToSQLNullString(lang.Primary()),
		SecondaryLanguage: mapping.ValueToSQLNullString(lang.Secondary()),
		Tin:               mapping.ValueToSQLNullString(entity.Tin().Value()),
		Pin:               mapping.ValueToSQLNullString(entity.Pin().Value()),
		Notes:             mapping.ValueToSQLNullString(entity.Notes()),
		BirthDate:         mapping.ValueToSQLNullTime(entity.BirthDate()),
		HireDate:          mapping.ValueToSQLNullTime(entity.HireDate()),
		ResignationDate:   mapping.PointerToSQLNullTime(entity.ResignationDate()),
	}
	return dbEmployee, dbEmployeeMeta
}

func ToDBUpload(upload *upload.Upload) *models.Upload {
	return &models.Upload{
		ID:        upload.ID,
		Path:      upload.Path,
		Hash:      upload.Hash,
		Size:      upload.Size,
		Mimetype:  upload.Mimetype.String(),
		CreatedAt: upload.CreatedAt,
		UpdatedAt: upload.UpdatedAt,
	}
}

func ToDomainUpload(dbUpload *models.Upload) *upload.Upload {
	var mime mimetype.MIME
	if dbUpload.Mimetype != "" {
		mime = *mimetype.Lookup(dbUpload.Mimetype)
	}
	return &upload.Upload{
		ID:        dbUpload.ID,
		Size:      dbUpload.Size,
		Path:      dbUpload.Path,
		Mimetype:  mime,
		CreatedAt: dbUpload.CreatedAt,
		UpdatedAt: dbUpload.UpdatedAt,
	}
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

func toDomainPosition(dbPosition *models.Position) (*position.Position, error) {
	return &position.Position{
		ID:          dbPosition.ID,
		Name:        dbPosition.Name,
		Description: dbPosition.Description,
		CreatedAt:   dbPosition.CreatedAt,
		UpdatedAt:   dbPosition.UpdatedAt,
	}, nil
}

func toDBPosition(position *position.Position) *models.Position {
	return &models.Position{
		ID:          position.ID,
		Name:        position.Name,
		Description: position.Description,
		CreatedAt:   position.CreatedAt,
		UpdatedAt:   position.UpdatedAt,
	}
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
