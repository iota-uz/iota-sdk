package shared

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"unicode"

	"github.com/iota-uz/iota-sdk/pkg/htmx"

	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
)

func Redirect(w http.ResponseWriter, r *http.Request, path string) {
	if htmx.IsHxRequest(r) {
		htmx.Redirect(w, path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}

func ParseID(r *http.Request) (uint, error) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		return 0, errors.Wrap(err, "Error parsing id")
	}
	return uint(id), nil
}

func SetFlash(w http.ResponseWriter, name string, value []byte) {
	c := &http.Cookie{Name: name, Value: base64.URLEncoding.EncodeToString(value)}
	http.SetCookie(w, c)
}

func SetFlashMap[K comparable, V any](w http.ResponseWriter, name string, value map[K]V) {
	errors, err := json.Marshal(value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	SetFlash(w, name, errors)
}

// GetInitials safely extracts the first character from first and last names,
// properly handling non-ASCII characters and converting to uppercase.
// Returns "NA" if both names are empty.
func GetInitials(firstName, lastName string) string {
	initials := ""
	if firstName != "" {
		firstRune := []rune(firstName)[0]
		initials += string(unicode.ToUpper(firstRune))
	}
	if lastName != "" {
		lastRune := []rune(lastName)[0]
		initials += string(unicode.ToUpper(lastRune))
	}
	if initials == "" {
		return "NA"
	}
	return initials
}
