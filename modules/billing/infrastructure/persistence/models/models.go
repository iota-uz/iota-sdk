package models

import (
	"encoding/json"
	"time"
)

type Transaction struct {
	ID        string
	TenantID  string
	Status    string
	Quantity  float64
	Currency  string
	Gateway   string
	Details   json.RawMessage
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ClickDetails struct {
	ServiceID         int64          `json:"service_id"`
	MerchantID        int64          `json:"merchant_id"`
	MerchantUserID    int64          `json:"merchant_user_id"`
	MerchantTransID   string         `json:"merchant_trans_id"`
	MerchantPrepareID int64          `json:"merchant_prepare_id"`
	MerchantConfirmID int64          `json:"merchant_confirm_id"`
	PayDocId          int64          `json:"pay_doc_id"`
	PaymentID         int64          `json:"payment_id"`
	PaymentStatus     int32          `json:"payment_status"`
	SignTime          string         `json:"sign_time"`
	SignString        string         `json:"sign_string"`
	ErrorCode         int32          `json:"error_code"`
	ErrorNote         string         `json:"error_note"`
	Link              string         `json:"link"`
	Params            map[string]any `json:"params"`
}

type PaymeReceiver struct {
	ID     string  `json:"id"`
	Amount float64 `json:"amount"`
}

type PaymeDetails struct {
	MerchantID  string          `json:"merchant_id"`
	ID          string          `json:"id"`
	Transaction string          `json:"transaction"`
	State       int32           `json:"state"`
	Time        int64           `json:"time"`
	CreatedTime int64           `json:"created_time"`
	PerformTime int64           `json:"perform_time"`
	CancelTime  int64           `json:"cancel_time"`
	Account     map[string]any  `json:"account"`
	Receivers   []PaymeReceiver `json:"receivers"`
	Additional  map[string]any  `json:"additional"`
	Reason      int32           `json:"reason"`
	ErrorCode   int32           `json:"error_code"`
	Link        string          `json:"link"`
	Params      map[string]any  `json:"params"`
}

type OctoDetails struct {
}

type StripeItem struct {
	PriceID  string `json:"price_id"`
	Quantity int64  `json:"quantity"`
}

type StripeDetails struct {
	Mode              string       `json:"mode"`
	BillingReason     string       `json:"billing_reason"`
	SessionID         string       `json:"session_id"`
	ClientReferenceID string       `json:"client_reference_id"`
	InvoiceID         string       `json:"invoice_id"`
	SubscriptionID    string       `json:"subscription_id"`
	CustomerID        string       `json:"customer_id"`
	Items             []StripeItem `json:"items"`
	SuccessURL        string       `json:"success_url"`
	CancelURL         string       `json:"cancel_url"`
	URL               string       `json:"url"`
}
