package eskiz

type Config interface {
	URL() string
	Email() string
	Password() string
}

func NewConfig(
	url string,
	email string,
	password string,
) Config {
	return &config{
		url:      url,
		email:    email,
		password: password,
	}
}

type config struct {
	url      string
	email    string
	password string
}

func (c *config) URL() string {
	return c.email
}

func (c *config) Email() string {
	return c.email
}

func (c *config) Password() string {
	return c.password
}
