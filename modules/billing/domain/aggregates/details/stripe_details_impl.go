package details

type StripeOption func(d *stripeDetails)

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

func NewStripeItem(priceID string, quantity int64) StripeItem {
	return &stripeItem{
		priceID:  priceID,
		quantity: quantity,
	}
}

type stripeItem struct {
	priceID  string
	quantity int64
}

func (i *stripeItem) PriceID() string { return i.priceID }
func (i *stripeItem) Quantity() int64 { return i.quantity }

func NewStripeDetails(clientReferenceID string, opts ...StripeOption) *stripeDetails {
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
func (d *stripeDetails) SuccessURL() string        { return d.successURL }
func (d *stripeDetails) CancelURL() string         { return d.cancelURL }
func (d *stripeDetails) URL() string               { return d.url }

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
