package configuration

import (
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/utils/fs"
	"github.com/joho/godotenv"
	"log"
	"regexp"
	"strconv"
	"time"
)

func LoadEnv() error {
	envExists := fs.FileExists(".env")
	envLocalExists := fs.FileExists(".env.local")

	if envExists && envLocalExists {
		return godotenv.Load(".env", ".env.local")
	}

	if envExists {
		return godotenv.Load(".env")
	}

	if envLocalExists {
		return godotenv.Load(".env.local")
	}
	return fmt.Errorf("no .env or .env.local file found")
}

func parseDuration(value, unit string) (time.Duration, error) {
	num, err := strconv.Atoi(value)
	if err != nil {
		log.Fatal("Error parsing SESSION_DURATION:", err)
	}
	switch unit {
	case "s":
		return time.Second * time.Duration(num), nil
	case "m":
		return time.Minute * time.Duration(num), nil
	case "h":
		return time.Hour * time.Duration(num), nil
	case "d":
		return time.Hour * 24 * time.Duration(num), nil
	case "w":
		return time.Hour * 24 * 7 * time.Duration(num), nil
	case "M":
		return time.Hour * 24 * 30 * time.Duration(num), nil
	case "y":
		return time.Hour * 24 * 365 * time.Duration(num), nil
	default:
		return 0, fmt.Errorf("invalid duration unit %s", unit)
	}
}

// ParseDuration parses a duration string and returns a time.Duration
// Supported units are s, m, h, d, w, M, y
// s - seconds
// m - minutes
// h - hours
// d - days
// w - weeks
// M - months
// y - years
// Example: 1h30m
// Example: 1y
// Example: 30d
// Example: 1w
// Example: 1M
func ParseDuration(d string) (time.Duration, error) {
	valueUnitRegex := regexp.MustCompile(`(\d+)([smhdwMy])`)
	matches := valueUnitRegex.FindAllStringSubmatch(d, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid duration format")
	}
	matched := ""
	for _, match := range matches {
		matched += match[0]
	}
	if matched != d {
		return 0, fmt.Errorf("invalid duration format")
	}
	var total time.Duration
	for _, match := range matches {
		if len(match) != 3 {
			return 0, fmt.Errorf("invalid duration format")
		}
		dur, err := parseDuration(match[1], match[2])
		if err != nil {
			return 0, err
		}
		total += dur
	}
	return total, nil
}
