package utils

import (
	"fmt"
	"github.com/joho/godotenv"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"log"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"time"
)

const (
	lowerCharSet    = "abcdedfghijklmnopqrst"
	upperCharSet    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	specialCharSet  = "!@#$%&*"
	numberSet       = "0123456789"
	alphaNumericSet = lowerCharSet + upperCharSet + numberSet
	allCharSet      = lowerCharSet + upperCharSet + specialCharSet + numberSet
)

func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func DirExists(dir string) bool {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func MkDirIfNone(dir string) error {
	if DirExists(dir) {
		return nil
	}
	return os.Mkdir(dir, 0755)
}

func RandomString(length int, specialCharacters bool) string {
	var password string
	for i := 0; i < length; i++ {
		if specialCharacters {
			random := rand.Intn(len(allCharSet))
			password += string(allCharSet[random])
		} else {
			random := rand.Intn(len(alphaNumericSet))
			password += string(alphaNumericSet[random])
		}
	}
	return password
}

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
	envExists := FileExists(".env")
	envLocalExists := FileExists(".env.local")
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

func Includes[T comparable](array []T, elem T) bool {
	for _, e := range array {
		if e == elem {
			return true
		}
	}
	return false
}

func Title(str string) string {
	return cases.Title(language.English, cases.NoLower).String(str)
}

func ReverseInPlace[T any](array []T) []T {
	length := len(array)
	swap := reflect.Swapper(array)
	for i := 0; i < length/2; i++ {
		swap(i, length-1-i)
	}
	return array
}

func Reverse[T any](array []T) []T {
	length := len(array)
	result := make([]T, length)
	for i, elem := range array {
		result[length-1-i] = elem
	}
	return result
}
