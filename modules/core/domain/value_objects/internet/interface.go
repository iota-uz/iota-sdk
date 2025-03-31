package internet

type Email interface {
	Value() string
	Domain() string
	Username() string
}

type IP interface {
	Value() string
	Version() IpVersion
}
