package schemas

import (
	"encoding/json"
	"fmt"
)

type PopulateRequest struct {
	Version string       `json:"version"`
	Tenant  *TenantSpec  `json:"tenant,omitempty"`
	Data    *DataSpec    `json:"data,omitempty"`
	Options *OptionsSpec `json:"options,omitempty"`
}

type TenantSpec struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Domain string `json:"domain"`
}

type DataSpec struct {
	Users     []UserSpec     `json:"users,omitempty"`
	Finance   *FinanceSpec   `json:"finance,omitempty"`
	CRM       *CRMSpec       `json:"crm,omitempty"`
	Warehouse *WarehouseSpec `json:"warehouse,omitempty"`
}

type UserSpec struct {
	Email       string   `json:"email"`
	Password    string   `json:"password"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	Permissions []string `json:"permissions,omitempty"`
	Language    string   `json:"language,omitempty"`
	Ref         string   `json:"_ref,omitempty"`
}

type FinanceSpec struct {
	MoneyAccounts     []MoneyAccountSpec    `json:"moneyAccounts,omitempty"`
	PaymentCategories []PaymentCategorySpec `json:"paymentCategories,omitempty"`
	ExpenseCategories []ExpenseCategorySpec `json:"expenseCategories,omitempty"`
	Payments          []PaymentSpec         `json:"payments,omitempty"`
	Expenses          []ExpenseSpec         `json:"expenses,omitempty"`
	Counterparties    []CounterpartySpec    `json:"counterparties,omitempty"`
	Debts             []DebtSpec            `json:"debts,omitempty"`
}

type MoneyAccountSpec struct {
	Name     string  `json:"name"`
	Currency string  `json:"currency"`
	Balance  float64 `json:"balance"`
	Type     string  `json:"type"`
	Ref      string  `json:"_ref,omitempty"`
}

type PaymentCategorySpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Ref  string `json:"_ref,omitempty"`
}

type ExpenseCategorySpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Ref  string `json:"_ref,omitempty"`
}

type AttachmentSpec struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
	MimeType string `json:"mimeType"`
}

type PaymentSpec struct {
	Amount      float64          `json:"amount"`
	Date        string           `json:"date"`
	AccountRef  string           `json:"accountRef"`
	CategoryRef string           `json:"categoryRef"`
	Comment     string           `json:"comment,omitempty"`
	Attachments []AttachmentSpec `json:"attachments,omitempty"`
	Ref         string           `json:"_ref,omitempty"`
}

type ExpenseSpec struct {
	Amount      float64          `json:"amount"`
	Date        string           `json:"date"`
	AccountRef  string           `json:"accountRef"`
	CategoryRef string           `json:"categoryRef"`
	Comment     string           `json:"comment,omitempty"`
	Attachments []AttachmentSpec `json:"attachments,omitempty"`
	Ref         string           `json:"_ref,omitempty"`
}

type CounterpartySpec struct {
	Name string `json:"name"`
	Type string `json:"type"`
	TIN  string `json:"tin,omitempty"`
	Ref  string `json:"_ref,omitempty"`
}

type DebtSpec struct {
	Amount          float64 `json:"amount"`
	Type            string  `json:"type"`
	CounterpartyRef string  `json:"counterpartyRef"`
	DueDate         string  `json:"dueDate,omitempty"`
	Ref             string  `json:"_ref,omitempty"`
}

type CRMSpec struct {
	Clients []ClientSpec `json:"clients,omitempty"`
}

type ClientSpec struct {
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
	Ref       string `json:"_ref,omitempty"`
}

type WarehouseSpec struct {
	Units    []UnitSpec    `json:"units,omitempty"`
	Products []ProductSpec `json:"products,omitempty"`
}

type UnitSpec struct {
	Title      string `json:"title"`
	ShortTitle string `json:"shortTitle"`
	Ref        string `json:"_ref,omitempty"`
}

type ProductSpec struct {
	Name    string  `json:"name"`
	UnitRef string  `json:"unitRef"`
	Price   float64 `json:"price"`
	Ref     string  `json:"_ref,omitempty"`
}

type OptionsSpec struct {
	ClearExisting      bool `json:"clearExisting,omitempty"`
	ReturnIds          bool `json:"returnIds,omitempty"`
	ValidateReferences bool `json:"validateReferences,omitempty"`
	StopOnError        bool `json:"stopOnError,omitempty"`
}

type PopulateResponse struct {
	Success bool                   `json:"success"`
	Message string                 `json:"message,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

func (r *PopulateRequest) Validate() error {
	if r.Version == "" {
		return fmt.Errorf("version is required")
	}

	if r.Data == nil {
		return fmt.Errorf("data is required")
	}

	return nil
}

func ParsePopulateRequest(data []byte) (*PopulateRequest, error) {
	var req PopulateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	return &req, nil
}
