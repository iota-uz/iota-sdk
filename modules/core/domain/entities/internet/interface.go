package internet

type Email interface {
	Value() string
}

type IP interface {
	Value() string
	Version() IpVersion
}
