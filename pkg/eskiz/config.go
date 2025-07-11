package eskiz

type ConfigOption func(s *config)

type Config interface {
	URL() string
	Email() string
	Password() string
	CallbackUrl() string
	MaxMessageSize() int
}

// WithCallbackUrl sets the callback URL for SMS notifications
func WithCallbackUrl(url string) ConfigOption {
	return func(c *config) {
		c.callbackUrl = url
	}
}

// WithMaxMessageSize sets the maximum message size limit
func WithMaxMessageSize(size int) ConfigOption {
	return func(c *config) {
		c.maxMessageSize = size
	}
}

func NewConfig(
	url string,
	email string,
	password string,
	opts ...ConfigOption,
) Config {
	cfg := &config{
		url:            url,
		email:          email,
		password:       password,
		maxMessageSize: 500,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

type config struct {
	url            string
	email          string
	password       string
	callbackUrl    string
	maxMessageSize int
}

func (c *config) URL() string {
	return c.url
}

func (c *config) Email() string {
	return c.email
}

func (c *config) Password() string {
	return c.password
}

func (c *config) CallbackUrl() string {
	return c.callbackUrl
}

func (c *config) MaxMessageSize() int {
	return c.maxMessageSize
}
