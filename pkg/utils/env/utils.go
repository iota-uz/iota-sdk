package env

import (
	"log"
	"os"
	"strconv"
)

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func MustGetEnv(key string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	log.Fatalf("Environment variable %s is not set", key)
	return ""
}

func GetEnvInt(key string, fallback int) (int, error) {
	if value, ok := os.LookupEnv(key); ok {
		return strconv.Atoi(value)
	}
	return fallback, nil
}

func MustGetEnvInt(key string, fallback int) int {
	if value, ok := os.LookupEnv(key); ok {
		i, err := strconv.Atoi(value)
		if err != nil {
			log.Fatalf("Environment variable %s is not an integer", key)
		}
		return i
	}
	return fallback
}
