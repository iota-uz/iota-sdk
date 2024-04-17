package helpers

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
)

func ValidateStruct(s interface{}) (err error) {
	// first make sure that the input is a struct
	// having any other type, especially a pointer to a struct,
	// might result in panic
	structType := reflect.TypeOf(s)
	if structType.Kind() != reflect.Struct {
		return errors.New("input param should be a struct")
	}

	// now go one by one through the fields and validate their value
	structVal := reflect.ValueOf(s)
	fieldNum := structVal.NumField()

	for i := 0; i < fieldNum; i++ {
		field := structVal.Field(i)
		fieldName := structType.Field(i).Name
		isSet := field.IsValid() && (!field.IsZero() || field.Kind() == reflect.Bool)

		if !isSet {
			err = errors.New(fmt.Sprintf("%v%s in not set; ", err, fieldName))
		}

	}
	return err
}

func BadRequest(w http.ResponseWriter, err error) {
	log.Println("Bad request:", err)
	w.WriteHeader(http.StatusBadRequest)
	w.Header().Set("Content-Type", "application/text")
	_, e := w.Write([]byte("Bad request: " + err.Error()))
	if e != nil {
		log.Println(e)
		return
	}
}

func ServerError(w http.ResponseWriter, err error) {
	log.Println("Server error:", err)
	w.WriteHeader(http.StatusInternalServerError)
	w.Header().Set("Content-Type", "application/text")
	_, e := w.Write([]byte("Server error: " + err.Error()))
	if e != nil {
		log.Println(e)
		return
	}
}

func NotFound(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusNotFound)
	w.Header().Set("Content-Type", "application/text")
	_, e := w.Write([]byte("Not found: " + err.Error()))
	if e != nil {
		log.Println(e)
		return
	}
}

func NotAuthorized(w http.ResponseWriter, err error) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Header().Set("Content-Type", "application/text")
	_, e := w.Write([]byte("Not authorized: " + err.Error()))
	if e != nil {
		log.Println(e)
		return
	}
}

func RespondWithJson(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Println(err)
		return
	}
}

func ServeTemplate(w http.ResponseWriter, name string, data interface{}) {
	files, err := ReadTemplates("templates")
	if err != nil {
		BadRequest(w, err)
		return
	}
	tmpl, err := template.New(name).ParseFiles(files...)
	if err != nil {
		BadRequest(w, err)
		return
	}
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		BadRequest(w, err)
		return
	}
}

func ReadTemplates(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range entries {
		if f.IsDir() {
			subFiles, err := ReadTemplates(filepath.Join(dir, f.Name()))
			if err != nil {
				return nil, err
			}
			files = append(files, subFiles...)
		} else {
			files = append(files, filepath.Join(dir, f.Name()))
		}
	}
	return files, nil
}

func RequireAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authenticated := r.Context().Value("authenticated")
		if authenticated == nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}
