package persistence

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/go-faster/errors"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/group"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/role"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/authlog"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/currency"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/session"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tab"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/country"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/general"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence/models"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

func ToDomainUser(dbUser *models.User, dbUpload *models.Upload, roles []role.Role, groupIDs []uuid.UUID, permissions []*permission.Permission) (user.User, error) {
	var avatar upload.Upload
	if dbUpload != nil {
		var err error
		avatar, err = ToDomainUpload(dbUpload)
		if err != nil {
			return nil, err
		}
	}

	email, err := internet.NewEmail(dbUser.Email)
	if err != nil {
		return nil, err
	}

	options := []user.Option{
		user.WithID(dbUser.ID),
		user.WithTenantID(uuid.MustParse(dbUser.TenantID)),
		user.WithMiddleName(dbUser.MiddleName.String),
		user.WithPassword(dbUser.Password.String),
		user.WithRoles(roles),
		user.WithGroupIDs(groupIDs),
		user.WithLastIP(dbUser.LastIP.String),
		user.WithLastLogin(dbUser.LastLogin.Time),
		user.WithLastAction(dbUser.LastAction.Time),
		user.WithCreatedAt(dbUser.CreatedAt),
		user.WithUpdatedAt(dbUser.UpdatedAt),
	}

	if permissions != nil {
		options = append(options, user.WithPermissions(permissions))
	}

	if dbUpload != nil {
		options = append(options, user.WithAvatar(avatar))
	}

	// Add phone if available
	if dbUser.Phone.Valid && dbUser.Phone.String != "" {
		phoneObj, err := phone.NewFromE164(dbUser.Phone.String)
		if err != nil {
			return nil, err
		}
		options = append(options, user.WithPhone(phoneObj))
	}

	return user.New(
		dbUser.FirstName,
		dbUser.LastName,
		email,
		user.UILanguage(dbUser.UILanguage),
		options...,
	), nil
}

func toDBUser(entity user.User) (*models.User, []*models.Role) {
	roles := make([]*models.Role, len(entity.Roles()))
	for i, r := range entity.Roles() {
		dbRole, _ := toDBRole(r)
		roles[i] = dbRole
	}

	var phoneValue sql.NullString
	if entity.Phone() != nil {
		phoneValue = mapping.ValueToSQLNullString(entity.Phone().Value())
	}

	return &models.User{
		ID:         entity.ID(),
		TenantID:   entity.TenantID().String(),
		FirstName:  entity.FirstName(),
		LastName:   entity.LastName(),
		MiddleName: mapping.ValueToSQLNullString(entity.MiddleName()),
		Email:      entity.Email().Value(),
		Phone:      phoneValue,
		UILanguage: string(entity.UILanguage()),
		Password:   mapping.ValueToSQLNullString(entity.Password()),
		AvatarID:   mapping.ValueToSQLNullInt32(int32(entity.AvatarID())),
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
			return nil, errors.Wrap(err, "failed to cast to domain permission")
		}
		domainPermissions[i] = dP
	}

	tenantID, err := uuid.Parse(dbRole.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse uuid")
	}

	options := []role.Option{
		role.WithID(dbRole.ID),
		role.WithDescription(dbRole.Description.String),
		role.WithPermissions(domainPermissions),
		role.WithCreatedAt(dbRole.CreatedAt),
		role.WithUpdatedAt(dbRole.UpdatedAt),
		role.WithTenantID(tenantID),
	}
	return role.New(dbRole.Name, options...), nil
}

func toDBRole(entity role.Role) (*models.Role, []*models.Permission) {
	permissions := make([]*models.Permission, len(entity.Permissions()))
	for i, p := range entity.Permissions() {
		permissions[i] = toDBPermission(p)
	}
	return &models.Role{
		ID:          entity.ID(),
		TenantID:    entity.TenantID().String(),
		Name:        entity.Name(),
		Description: mapping.ValueToSQLNullString(entity.Description()),
		CreatedAt:   entity.CreatedAt(),
		UpdatedAt:   entity.UpdatedAt(),
	}, permissions
}

func toDBPermission(entity *permission.Permission) *models.Permission {
	return &models.Permission{
		ID:       entity.ID.String(),
		TenantID: entity.TenantID.String(),
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

	tenantID, err := uuid.Parse(dbPermission.TenantID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse uuid")
	}

	return &permission.Permission{
		ID:       id,
		TenantID: tenantID,
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
		TenantID:  upload.TenantID().String(),
		Path:      upload.Path(),
		Hash:      upload.Hash(),
		Name:      upload.Name(),
		Size:      upload.Size().Bytes(),
		Type:      upload.Type().String(),
		Mimetype:  upload.Mimetype().String(),
		CreatedAt: upload.CreatedAt(),
		UpdatedAt: upload.UpdatedAt(),
	}
}

func ToDomainUpload(dbUpload *models.Upload) (upload.Upload, error) {
	var mime *mimetype.MIME
	if dbUpload.Mimetype != "" {
		mime = mimetype.Lookup(dbUpload.Mimetype)
	}

	tenantID, err := uuid.Parse(dbUpload.TenantID)
	if err != nil {
		return nil, err
	}

	return upload.NewWithID(
		dbUpload.ID,
		tenantID,
		dbUpload.Hash,
		dbUpload.Path,
		dbUpload.Name,
		dbUpload.Size,
		mime,
		upload.UploadType(dbUpload.Type),
		dbUpload.CreatedAt,
		dbUpload.UpdatedAt,
	), nil
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
		TenantID: tab.TenantID.String(),
	}
}

func ToDomainTab(dbTab *models.Tab) (*tab.Tab, error) {
	tenantID, err := uuid.Parse(dbTab.TenantID)
	if err != nil {
		return nil, err
	}

	return &tab.Tab{
		ID:       dbTab.ID,
		Href:     dbTab.Href,
		Position: dbTab.Position,
		UserID:   dbTab.UserID,
		TenantID: tenantID,
	}, nil
}

func ToDBSession(session *session.Session) *models.Session {
	return &models.Session{
		UserID:    session.UserID,
		TenantID:  session.TenantID.String(),
		Token:     session.Token,
		IP:        session.IP,
		UserAgent: session.UserAgent,
		CreatedAt: session.CreatedAt,
		ExpiresAt: session.ExpiresAt,
	}
}

func ToDomainSession(dbSession *models.Session) *session.Session {
	tenantID, err := uuid.Parse(dbSession.TenantID)
	if err != nil {
		tenantID = uuid.Nil
	}

	return &session.Session{
		UserID:    dbSession.UserID,
		TenantID:  tenantID,
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
		TenantID:  log.TenantID.String(),
		UserID:    log.UserID,
		IP:        log.IP,
		UserAgent: log.UserAgent,
		CreatedAt: log.CreatedAt,
	}
}

func toDomainAuthenticationLog(dbLog *models.AuthenticationLog) *authlog.AuthenticationLog {
	tenantID, err := uuid.Parse(dbLog.TenantID)
	if err != nil {
		tenantID = uuid.Nil
	}

	return &authlog.AuthenticationLog{
		ID:        dbLog.ID,
		TenantID:  tenantID,
		UserID:    dbLog.UserID,
		IP:        dbLog.IP,
		UserAgent: dbLog.UserAgent,
		CreatedAt: dbLog.CreatedAt,
	}
}

// Passport mappers
func ToDomainPassport(dbPassport *models.Passport) (passport.Passport, error) {
	id, err := uuid.Parse(dbPassport.ID)
	if err != nil {
		return nil, err
	}

	tenantID, err := uuid.Parse(dbPassport.TenantID)
	if err != nil {
		return nil, err
	}

	opts := []passport.Option{
		passport.WithID(id),
		passport.WithTenantID(tenantID),
	}

	if dbPassport.FirstName.Valid || dbPassport.LastName.Valid || dbPassport.MiddleName.Valid {
		opts = append(opts, passport.WithFullName(
			dbPassport.FirstName.String,
			dbPassport.LastName.String,
			dbPassport.MiddleName.String,
		))
	}

	if dbPassport.Gender.Valid {
		g, err := general.NewGender(dbPassport.Gender.String)
		if err != nil {
			return nil, err
		}
		opts = append(opts, passport.WithGender(g))
	}

	if dbPassport.BirthDate.Valid || dbPassport.BirthPlace.Valid {
		birthDate := time.Time{}
		if dbPassport.BirthDate.Valid {
			birthDate = dbPassport.BirthDate.Time
		}

		birthPlace := ""
		if dbPassport.BirthPlace.Valid {
			birthPlace = dbPassport.BirthPlace.String
		}

		opts = append(opts, passport.WithBirthDetails(birthDate, birthPlace))
	}

	if dbPassport.Nationality.Valid {
		opts = append(opts, passport.WithNationality(dbPassport.Nationality.String))
	}

	if dbPassport.PassportType.Valid {
		opts = append(opts, passport.WithPassportType(dbPassport.PassportType.String))
	}

	if dbPassport.IssuedAt.Valid {
		opts = append(opts, passport.WithIssuedAt(dbPassport.IssuedAt.Time))
	}

	if dbPassport.IssuedBy.Valid {
		opts = append(opts, passport.WithIssuedBy(dbPassport.IssuedBy.String))
	}

	if dbPassport.IssuingCountry.Valid {
		opts = append(opts, passport.WithIssuingCountry(dbPassport.IssuingCountry.String))
	}

	if dbPassport.ExpiresAt.Valid {
		opts = append(opts, passport.WithExpiresAt(dbPassport.ExpiresAt.Time))
	}

	if dbPassport.MachineReadableZone.Valid {
		opts = append(opts, passport.WithMachineReadableZone(dbPassport.MachineReadableZone.String))
	}

	if len(dbPassport.BiometricData) > 0 {
		// In a real implementation, you would parse the json from bytes to map
		var bioMap map[string]interface{}
		if err := json.Unmarshal(dbPassport.BiometricData, &bioMap); err != nil {
			return nil, err
		}
		opts = append(opts, passport.WithBiometricData(bioMap))
	}

	if len(dbPassport.SignatureImage) > 0 {
		opts = append(opts, passport.WithSignatureImage(dbPassport.SignatureImage))
	}

	if dbPassport.Remarks.Valid {
		opts = append(opts, passport.WithRemarks(dbPassport.Remarks.String))
	}

	// Create the passport with the series, number and all the options
	series := ""
	if dbPassport.Series.Valid {
		series = dbPassport.Series.String
	}

	number := ""
	if dbPassport.PassportNumber.Valid {
		number = dbPassport.PassportNumber.String
	}

	return passport.New(series, number, opts...), nil
}

func ToDBPassport(passportEntity passport.Passport) (*models.Passport, error) {
	var biometricDataJSON []byte
	if passportEntity.BiometricData() != nil {
		var err error
		biometricDataJSON, err = json.Marshal(passportEntity.BiometricData())
		if err != nil {
			return nil, err
		}
	}

	var gender sql.NullString
	if passportEntity.Gender() != nil {
		gender = mapping.ValueToSQLNullString(passportEntity.Gender().String())
	}

	return &models.Passport{
		ID:                  passportEntity.ID().String(),
		TenantID:            passportEntity.TenantID().String(),
		FirstName:           mapping.ValueToSQLNullString(passportEntity.FirstName()),
		LastName:            mapping.ValueToSQLNullString(passportEntity.LastName()),
		MiddleName:          mapping.ValueToSQLNullString(passportEntity.MiddleName()),
		Gender:              gender,
		BirthDate:           mapping.ValueToSQLNullTime(passportEntity.BirthDate()),
		BirthPlace:          mapping.ValueToSQLNullString(passportEntity.BirthPlace()),
		Nationality:         mapping.ValueToSQLNullString(passportEntity.Nationality()),
		PassportType:        mapping.ValueToSQLNullString(passportEntity.PassportType()),
		PassportNumber:      mapping.ValueToSQLNullString(passportEntity.Number()),
		Series:              mapping.ValueToSQLNullString(passportEntity.Series()),
		IssuingCountry:      mapping.ValueToSQLNullString(passportEntity.IssuingCountry()),
		IssuedAt:            mapping.ValueToSQLNullTime(passportEntity.IssuedAt()),
		IssuedBy:            mapping.ValueToSQLNullString(passportEntity.IssuedBy()),
		ExpiresAt:           mapping.ValueToSQLNullTime(passportEntity.ExpiresAt()),
		MachineReadableZone: mapping.ValueToSQLNullString(passportEntity.MachineReadableZone()),
		BiometricData:       biometricDataJSON,
		SignatureImage:      passportEntity.SignatureImage(),
		Remarks:             mapping.ValueToSQLNullString(passportEntity.Remarks()),
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}, nil
}

func ToDomainGroup(dbGroup *models.Group, users []user.User, roles []role.Role) (group.Group, error) {
	groupID, err := uuid.Parse(dbGroup.ID)
	if err != nil {
		return nil, err
	}

	tenantID, err := uuid.Parse(dbGroup.TenantID)
	if err != nil {
		return nil, err
	}

	opts := []group.Option{
		group.WithID(groupID),
		group.WithTenantID(tenantID),
		group.WithDescription(dbGroup.Description.String),
		group.WithUsers(users),
		group.WithRoles(roles),
		group.WithCreatedAt(dbGroup.CreatedAt),
		group.WithUpdatedAt(dbGroup.UpdatedAt),
	}

	return group.New(dbGroup.Name, opts...), nil
}

func ToDBGroup(g group.Group) *models.Group {
	return &models.Group{
		ID:          g.ID().String(),
		TenantID:    g.TenantID().String(),
		Name:        g.Name(),
		Description: mapping.ValueToSQLNullString(g.Description()),
		CreatedAt:   g.CreatedAt(),
		UpdatedAt:   g.UpdatedAt(),
	}
}
