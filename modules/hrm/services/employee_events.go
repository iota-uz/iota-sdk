package services

import (
	"context"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/hrm/domain/aggregates/employee"
	"github.com/iota-uz/iota-sdk/modules/hrm/permissions"
	"github.com/iota-uz/iota-sdk/modules/logging/history"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/data"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/events"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// Versioned IDs of the employee events.
const (
	EmployeeCreatedEventID = "hrm.employee.created.v1"
	EmployeeUpdatedEventID = "hrm.employee.updated.v1"
)

// NativeEmployee is the ID of the hrm.employee native entity.
const NativeEmployee = "hrm.employee"

func localized(en, ru, uz, uzCyrl string) manifest.Label {
	return manifest.Label{"en": en, "ru": ru, "uz": uz, "uz-Cyrl": uzCyrl}
}

// EmployeeEntity describes modules/hrm `employees` (key: bigint) for the
// extension runtime. Salary, tax and personal identification numbers and the
// birth date are deliberately not exposed.
func EmployeeEntity() data.NativeEntity {
	str := func(n int) manifest.Type { return manifest.Type{Kind: manifest.KindString, MaxLength: n} }
	opt := func(n int) manifest.Type {
		return manifest.Type{Kind: manifest.KindString, MaxLength: n, Optional: true, Nullable: true}
	}
	return data.NativeEntity{
		ID: NativeEmployee, Table: "employees", KeyColumn: "id", KeyType: data.KeyInt64, TenantColumn: "tenant_id",
		ReadPermission: permissions.EmployeeRead.Name(),
		Label:          localized("Employee", "Сотрудник", "Xodim", "Ходим"),
		Fields: []data.NativeField{
			{ID: "firstName", Column: "first_name", Type: str(255)},
			{ID: "lastName", Column: "last_name", Type: str(255)},
			{ID: "middleName", Column: "middle_name", Type: opt(255)},
			{ID: "email", Column: "email", Type: str(255)},
			{ID: "phone", Column: "phone", Type: opt(255)},
			{ID: "createdAt", Column: "created_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
			{ID: "updatedAt", Column: "updated_at", Type: manifest.Type{Kind: manifest.KindDateTime, Optional: true, Nullable: true}},
		},
	}
}

// EmployeeCreatedPayload is the payload of hrm.employee.created.v1. Salary,
// tax and personal identification numbers and the birth date never travel.
type EmployeeCreatedPayload struct {
	EmployeeID int64   `json:"employeeId,string" wire:"int64"`
	FirstName  string  `json:"firstName"`
	LastName   string  `json:"lastName"`
	Email      string  `json:"email"`
	Phone      *string `json:"phone,omitempty"`
	HireDate   string  `json:"hireDate" wire:"date"`
}

// EmployeeUpdatedPayload is the payload of hrm.employee.updated.v1: the IDs of
// the changed fields (never their values) and the resignation date.
type EmployeeUpdatedPayload struct {
	EmployeeID      int64    `json:"employeeId,string" wire:"int64"`
	Changed         []string `json:"changed"`
	ResignationDate *string  `json:"resignationDate,omitempty" wire:"date"`
}

var employeeSensitive = map[string][]string{"email": {permissions.EmployeeRead.Name()}, "phone": {permissions.EmployeeRead.Name()}}

// EmployeeCreatedEvent is published in the transaction that creates an employee.
var EmployeeCreatedEvent = native.Event[EmployeeCreatedPayload](EmployeeCreatedEventID, native.EventSpec{
	Subject:     NativeEmployee,
	Label:       localized("Employee created", "Сотрудник создан", "Xodim yaratildi", "Ходим яратилди"),
	Description: "An employee was created. Published in the transaction that creates the employee.",
	Descriptions: localized(
		"An employee was created.",
		"Создан сотрудник.",
		"Xodim yaratildi.",
		"Ходим яратилди."),
	ReadPermission: permissions.EmployeeRead.Name(),
	Sensitive:      employeeSensitive,
	Category:       "hrm.employees",
})

// EmployeeUpdatedEvent is published in the transaction of an update that
// changed an employee.
var EmployeeUpdatedEvent = native.Event[EmployeeUpdatedPayload](EmployeeUpdatedEventID, native.EventSpec{
	Subject:     NativeEmployee,
	Label:       localized("Employee updated", "Сотрудник изменён", "Xodim o'zgartirildi", "Ходим ўзгартирилди"),
	Description: "An employee was updated. Published in the transaction of the update; changed lists the field IDs it changed, without their values.",
	Descriptions: localized(
		"An employee was updated; the changed fields are listed.",
		"Сотрудник изменён; перечислены изменённые поля.",
		"Xodim o'zgartirildi; o'zgargan maydonlar ko'rsatiladi.",
		"Ходим ўзгартирилди; ўзгарган майдонлар кўрсатилади."),
	ReadPermission: permissions.EmployeeRead.Name(),
	Category:       "hrm.employees",
})

// Natives is the HRM services' native registration: the hrm.employee entity
// and the employee events. The HRM module contributes it (hrm.Natives).
func Natives(s *native.Set) error {
	return s.Add(native.Entity(EmployeeEntity()), EmployeeCreatedEvent, EmployeeUpdatedEvent)
}

func employeeRef(id uint) *spi.ObjectRef {
	return &spi.ObjectRef{Entity: NativeEmployee, Key: strconv.FormatUint(uint64(id), 10)}
}

// employeeEntityFields maps the history snapshot keys of an employee to the
// field IDs a changed list may name. Fields outside the entity projection are
// listed too, their values are not.
var employeeEntityFields = []struct{ snapshot, field string }{
	{"first_name", "firstName"}, {"last_name", "lastName"}, {"middle_name", "middleName"},
	{"email", "email"}, {"phone", "phone"}, {"hire_date", "hireDate"}, {"resignation_date", "resignationDate"},
	{"birth_date", "birthDate"}, {"salary", "salary"}, {"tin", "tin"}, {"pin", "pin"},
	{"primary_language", "primaryLanguage"}, {"secondary_language", "secondaryLanguage"},
	{"notes", "notes"}, {"avatar_id", "avatarId"},
}

// employeeEventValues are the values a changed list compares: the history
// snapshot plus the persisted fields it does not carry.
func employeeEventValues(e employee.Employee) history.Snapshot {
	values := employeeSnapshot(e)
	values["primary_language"], values["secondary_language"] = "", ""
	if lang := e.Language(); lang != nil {
		values["primary_language"], values["secondary_language"] = lang.Primary(), lang.Secondary()
	}
	values["notes"] = e.Notes()
	values["avatar_id"] = e.AvatarID()
	return values
}

func employeeChangedFields(before, after history.Snapshot) []string {
	out := []string{}
	for _, f := range employeeEntityFields {
		if !reflect.DeepEqual(before[f.snapshot], after[f.snapshot]) {
			out = append(out, f.field)
		}
	}
	slices.Sort(out)
	return out
}

func dateOf(t time.Time) string { return t.Format(time.DateOnly) }

// publishEmployeeCreated enqueues hrm.employee.created.v1.
func publishEmployeeCreated(ctx context.Context, created employee.Employee) error {
	p := EmployeeCreatedPayload{
		EmployeeID: int64(created.ID()), FirstName: created.FirstName(), LastName: created.LastName(),
		HireDate: dateOf(created.HireDate()),
	}
	if created.Email() != nil {
		p.Email = created.Email().Value()
	}
	if phone := created.Phone(); phone != "" {
		p.Phone = &phone
	}
	return serrors.Wrap("EmployeeService.publishEmployeeCreated", events.PublishInTx(ctx, created.TenantID(), EmployeeCreatedEvent, employeeRef(created.ID()), p))
}

// publishEmployeeUpdated enqueues hrm.employee.updated.v1 when the update
// changed a field; an update that changed nothing publishes nothing.
func publishEmployeeUpdated(ctx context.Context, before, after employee.Employee) error {
	changed := employeeChangedFields(employeeEventValues(before), employeeEventValues(after))
	if len(changed) == 0 {
		return nil
	}
	p := EmployeeUpdatedPayload{EmployeeID: int64(after.ID()), Changed: changed}
	if after.ResignationDate() != nil {
		d := dateOf(*after.ResignationDate())
		p.ResignationDate = &d
	}
	return serrors.Wrap("EmployeeService.publishEmployeeUpdated", events.PublishInTx(ctx, after.TenantID(), EmployeeUpdatedEvent, employeeRef(after.ID()), p))
}
