package utils

import (
	"fmt"
	"github.com/iota-agency/iota-erp/sdk/utils/fs"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"time"
)

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func DbOpts() string {
	return fmt.Sprintf("host=%s port=%s user=%s dbname=%s password=%s sslmode=disable",
		GetEnv("DB_HOST", "localhost"), GetEnv("DB_PORT", "5432"),
		GetEnv("DB_USER", "postgres"),
		GetEnv("DB_NAME", "iota_erp"), GetEnv("DB_PASSWORD", "postgres"))
}

func LoadEnv() {
	envExists := fs.FileExists(".env")
	envLocalExists := fs.FileExists(".env.local")
	var err error
	if envExists && envLocalExists {
		err = godotenv.Load(".env", ".env.local")
	} else if envExists {
		err = godotenv.Load(".env")
	} else if envLocalExists {
		err = godotenv.Load(".env.local")
	} else {
		err = fmt.Errorf("no .env or .env.local file found")
	}
	if err != nil {
		log.Fatal("Error loading .env file:", err)
	}
}

func SessionDuration() time.Duration {
	d := GetEnv("SESSION_DURATION", "30d")
	num, err := strconv.Atoi(d[:len(d)-1])
	if err != nil {
		log.Fatal("Error parsing SESSION_DURATION:", err)
	}
	unit := d[len(d)-1:]
	switch unit {
	case "s":
		return time.Second * time.Duration(num)
	case "m":
		return time.Minute * time.Duration(num)
	case "h":
		return time.Hour * time.Duration(num)
	case "d":
		return time.Hour * 24 * time.Duration(num)
	case "w":
		return time.Hour * 24 * 7 * time.Duration(num)
	case "M":
		return time.Hour * 24 * 30 * time.Duration(num)
	case "y":
		return time.Hour * 24 * 365 * time.Duration(num)
	default:
		log.Fatal("Error parsing JWT_DURATION: invalid unit")
	}
	return 0
}
