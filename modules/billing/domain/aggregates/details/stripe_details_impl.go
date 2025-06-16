package details

type StripeOption func(d *stripeDetails)
type StripeItemOption func(i *stripeItem)
type StripeSubscriptionDataOption func(d *stripeSubscriptionData)

func StripeWithMode(mode string) StripeOption {
	return func(d *stripeDetails) {
		d.mode = mode
	}
}

func StripeWithBillingReason(reason string) StripeOption {
	return func(d *stripeDetails) {
		d.billingReason = reason
	}
}

func StripeWithSessionID(sessionID string) StripeOption {
	return func(d *stripeDetails) {
		d.sessionID = sessionID
	}
}

func StripeWithClientReferenceID(clientReferenceID string) StripeOption {
	return func(d *stripeDetails) {
		d.clientReferenceID = clientReferenceID
	}
}

func StripeWithInvoiceID(invoiceID string) StripeOption {
	return func(d *stripeDetails) {
		d.invoiceID = invoiceID
	}
}

func StripeWithSubscriptionID(subscriptionID string) StripeOption {
	return func(d *stripeDetails) {
		d.subscriptionID = subscriptionID
	}
}

func StripeWithCustomerID(customerID string) StripeOption {
	return func(d *stripeDetails) {
		d.customerID = customerID
	}
}

func StripeWithItems(items []StripeItem) StripeOption {
	return func(d *stripeDetails) {
		d.items = items
	}
}

func StripeWithSubscription(subscriptionData StripeSubscriptionData) StripeOption {
	return func(d *stripeDetails) {
		d.subscriptionData = subscriptionData
	}
}

func StripeWithSuccessURL(successURL string) StripeOption {
	return func(d *stripeDetails) {
		d.successURL = successURL
	}
}

func StripeWithCancelURL(cancelURL string) StripeOption {
	return func(d *stripeDetails) {
		d.cancelURL = cancelURL
	}
}

func StripeWithURL(url string) StripeOption {
	return func(d *stripeDetails) {
		d.url = url
	}
}

func StripeItemWithAdjustableQuantity(
	enabled bool,
	minimum int64,
	maximum int64,
) StripeItemOption {
	return func(i *stripeItem) {
		i.adjustableQuantity = &stripeItemAdjustableQuantity{
			enabled: enabled,
			minimum: minimum,
			maximum: maximum,
		}
	}
}

func NewStripeItem(
	priceID string,
	quantity int64,
	opts ...StripeItemOption,
) StripeItem {
	i := &stripeItem{
		priceID:            priceID,
		quantity:           quantity,
		adjustableQuantity: nil,
	}

	for _, opt := range opts {
		opt(i)
	}

	return i
}

type stripeItem struct {
	adjustableQuantity StripeItemAdjustableQuantity
	priceID            string
	quantity           int64
}

func (i *stripeItem) AdjustableQuantity() StripeItemAdjustableQuantity {
	return i.adjustableQuantity
}

func (i *stripeItem) PriceID() string { return i.priceID }
func (i *stripeItem) Quantity() int64 { return i.quantity }

type stripeItemAdjustableQuantity struct {
	enabled bool
	minimum int64
	maximum int64
}

func (s *stripeItemAdjustableQuantity) Enabled() bool {
	return s.enabled
}

func (s *stripeItemAdjustableQuantity) Maximum() int64 {
	return s.maximum
}

func (s *stripeItemAdjustableQuantity) Minimum() int64 {
	return s.minimum
}

func StripeSubscriptionDataWithDescription(description string) StripeSubscriptionDataOption {
	return func(d *stripeSubscriptionData) {
		d.description = description
	}
}

func StripeSubscriptionDataWithTrialPeriodDays(trialPeriodDays int64) StripeSubscriptionDataOption {
	return func(d *stripeSubscriptionData) {
		d.trialPeriodDays = trialPeriodDays
	}
}

func NewStripeSubscriptionData(
	opts ...StripeSubscriptionDataOption,
) StripeSubscriptionData {
	sd := &stripeSubscriptionData{}

	for _, opt := range opts {
		opt(sd)
	}

	return sd
}

type stripeSubscriptionData struct {
	description     string
	trialPeriodDays int64
}

func (s stripeSubscriptionData) Description() string {
	return s.description
}

func (s stripeSubscriptionData) TrialPeriodDays() int64 {
	return s.trialPeriodDays
}

func NewStripeDetails(
	clientReferenceID string,
	opts ...StripeOption,
) *stripeDetails {
	d := &stripeDetails{
		clientReferenceID: clientReferenceID,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type stripeDetails struct {
	mode              string
	billingReason     string
	sessionID         string
	clientReferenceID string
	invoiceID         string
	subscriptionID    string
	customerID        string
	items             []StripeItem
	subscriptionData  StripeSubscriptionData
	successURL        string
	cancelURL         string
	url               string
}

// Getters

func (d *stripeDetails) Mode() string              { return d.mode }
func (d *stripeDetails) BillingReason() string     { return d.billingReason }
func (d *stripeDetails) SessionID() string         { return d.sessionID }
func (d *stripeDetails) ClientReferenceID() string { return d.clientReferenceID }
func (d *stripeDetails) InvoiceID() string         { return d.invoiceID }
func (d *stripeDetails) SubscriptionID() string    { return d.subscriptionID }
func (d *stripeDetails) CustomerID() string        { return d.customerID }
func (d *stripeDetails) Items() []StripeItem       { return d.items }
func (d *stripeDetails) SubscriptionData() StripeSubscriptionData {
	return d.subscriptionData
}
func (d *stripeDetails) SuccessURL() string { return d.successURL }
func (d *stripeDetails) CancelURL() string  { return d.cancelURL }
func (d *stripeDetails) URL() string        { return d.url }

// Setters

func (d *stripeDetails) SetMode(mode string) StripeDetails {
	result := *d
	result.mode = mode
	return &result
}

func (d *stripeDetails) SetBillingReason(reason string) StripeDetails {
	result := *d
	result.billingReason = reason
	return &result
}

func (d *stripeDetails) SetSessionID(sessionID string) StripeDetails {
	result := *d
	result.sessionID = sessionID
	return &result
}

func (d *stripeDetails) SetClientReferenceID(clientReferenceID string) StripeDetails {
	result := *d
	result.clientReferenceID = clientReferenceID
	return &result
}

func (d *stripeDetails) SetInvoiceID(invoiceID string) StripeDetails {
	result := *d
	result.invoiceID = invoiceID
	return &result
}

func (d *stripeDetails) SetSubscriptionID(subscriptionID string) StripeDetails {
	result := *d
	result.subscriptionID = subscriptionID
	return &result
}

func (d *stripeDetails) SetCustomerID(customerID string) StripeDetails {
	result := *d
	result.customerID = customerID
	return &result
}

func (d *stripeDetails) SetItems(items []StripeItem) StripeDetails {
	result := *d
	result.items = items
	return &result
}

func (d *stripeDetails) SetSuccessURL(successURL string) StripeDetails {
	result := *d
	result.successURL = successURL
	return &result
}

func (d *stripeDetails) SetCancelURL(cancelURL string) StripeDetails {
	result := *d
	result.cancelURL = cancelURL
	return &result
}

func (d *stripeDetails) SetURL(url string) StripeDetails {
	result := *d
	result.url = url
	return &result
}
