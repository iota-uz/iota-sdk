package internet

import (
	"errors"
	"strconv"
	"strings"
)

var (
	ErrInvalidIP = errors.New("invalid email")
)

type IpVersion string

const (
	IPv4 IpVersion = "IPv4"
	IPv6 IpVersion = "IPv6"
)

func NewIP(v string, version IpVersion) (IP, error) {
	if !IsValidIP(v, version) {
		return nil, ErrInvalidIP
	}
	return &ip{
		value:   v,
		version: version,
	}, nil
}

type ip struct {
	value   string
	version IpVersion
}

func (i *ip) Value() string {
	return i.value
}

func (i *ip) Version() IpVersion {
	return i.version
}

func isValidIPv4(value string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return false
	}

	for _, part := range parts {
		// Check for leading zeros
		if len(part) > 1 && part[0] == '0' {
			return false
		}

		// Parse and validate number
		num, err := strconv.Atoi(part)
		if err != nil || num < 0 || num > 255 {
			return false
		}
	}

	return true
}

func isValidIPv6(value string) bool {
	parts := strings.Split(value, ":")
	if len(parts) != 8 {
		return false
	}

	for _, part := range parts {
		if len(part) == 0 || len(part) > 4 {
			return false
		}

		// Validate hex values
		for _, c := range part {
			isHex := (c >= '0' && c <= '9') ||
				(c >= 'a' && c <= 'f') ||
				(c >= 'A' && c <= 'F')
			if !isHex {
				return false
			}
		}
	}

	return true
}

func IsValidIP(value string, version IpVersion) bool {
	if version == IPv4 {
		return isValidIPv4(value)
	}
	return isValidIPv6(value)
}
