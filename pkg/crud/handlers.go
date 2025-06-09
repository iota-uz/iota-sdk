package crud

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	formui "github.com/iota-uz/iota-sdk/components/scaffold/form"
	sfui "github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
)

// parseFormToMap extracts form values into a map
func parseFormToMap(r *http.Request) (map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for key, values := range r.Form {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}

	return result, nil
}

// HTTP Handlers
func (s *Schema[T, ID]) listHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	var params FindParams
	// sort (format: column:asc|desc)
	//if v := r.URL.Query().Get("sort"); v != "" {
	//	parts := strings.Split(v, ":")
	//	if len(parts) == 2 {
	//		params.SortBy = repo.SortBy[string]{
	//			Fields:    []string{parts[0]},
	//			Ascending: parts[1] == "asc",
	//		}
	//	}
	//}
	// search
	params.Search = r.URL.Query().Get("search")
	// (Additional Filters could be appended here...)

	// Fetch data
	items, err := s.Service.List(ctx, params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build table config
	tcfg := sfui.NewTableConfig(s.Service.Name, s.Service.Path)
	// Add columns based on schema Fields
	for _, f := range s.Service.Fields {
		tcfg.AddCols(
			sfui.Column(f.Key(), f.Label()),
		)
	}

	// Add rows
	for _, item := range items {
		// Prepare cell components
		cells := make([]templ.Component, 0, len(s.Service.Fields))
		rv := reflect.ValueOf(item)
		if rv.Kind() == reflect.Ptr {
			rv = rv.Elem()
		}
		for _, f := range s.Service.Fields {
			fv := rv.FieldByNameFunc(func(name string) bool {
				return strings.EqualFold(name, f.Key())
			})
			val := ""
			if fv.IsValid() {
				val = fmt.Sprint(fv.Interface())
			}
			cells = append(cells, templ.Raw(val))
		}
		// Construct drawer URL for edit
		idVal := reflect.ValueOf(item)
		if idVal.Kind() == reflect.Ptr {
			idVal = idVal.Elem()
		}
		idField := idVal.FieldByName(s.Service.IDField)
		url := fmt.Sprintf("%s/%v/edit", s.Service.Path, idField.Interface())

		tcfg.AddRows(
			sfui.Row(cells...).ApplyOpts(sfui.WithDrawer(url)),
		)
	}

	// Render table or rows for HTMX
	if htmx.IsHxRequest(r) {
		s.Renderer(w, r, sfui.Rows(tcfg))
	} else {
		s.Renderer(w, r, sfui.Page(tcfg))
	}
}

func (s *Schema[T, ID]) newHandler(w http.ResponseWriter, r *http.Request) {
	cfg := formui.NewFormConfig("New "+s.Service.Name, s.Service.Path, s.Service.Path, "Create").
		Add(s.Service.Fields...)
	s.Renderer(w, r, formui.Page(cfg), templ.WithStreaming())
}

func (s *Schema[T, ID]) createHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	formData, err := parseFormToMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err = s.Service.CreateEntity(ctx, formData)
	if err != nil {
		if errors.Is(err, ErrValidation) {
			http.Error(w, err.Error(), http.StatusBadRequest)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Redirect to the list view
	http.Redirect(w, r, s.Service.Path, http.StatusSeeOther)
}

func (s *Schema[T, ID]) editHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id"]

	// Parse the ID
	idVal, err := parseID[ID](idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid ID: %v", err), http.StatusBadRequest)
		return
	}

	// Fetch the entity
	entity, err := s.Service.Get(ctx, idVal)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	// Extract field values from entity
	fieldValues := s.Service.Extract(entity)

	// Create fields with values
	fields := make([]formui.Field, len(s.Service.Fields))
	for i, field := range s.Service.Fields {
		switch f := field.(type) {
		case formui.TextField:
			if val, ok := fieldValues[field.Key()]; ok {
				fields[i] = f.WithValue(val)
			} else {
				fields[i] = field
			}
		default:
			fields[i] = field
		}
	}

	// Create form with entity values
	formAction := fmt.Sprintf("%s/%v", s.Service.Path, idVal)
	cfg := formui.NewFormConfig("Edit "+s.Service.Name, formAction, s.Service.Path, "Update").
		WithMethod("PUT").
		Add(fields...)

	s.Renderer(w, r, formui.Page(cfg), templ.WithStreaming())
}

func (s *Schema[T, ID]) updateHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id"]

	// Parse the ID
	idVal, err := parseID[ID](idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid ID: %v", err), http.StatusBadRequest)
		return
	}

	formData, err := parseFormToMap(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = s.Service.UpdateEntity(ctx, idVal, formData)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrValidation) {
			status = http.StatusBadRequest
		} else if errors.Is(err, ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	// Redirect to the list view
	http.Redirect(w, r, s.Service.Path, http.StatusSeeOther)
}

func (s *Schema[T, ID]) deleteHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := mux.Vars(r)["id"]

	// parse the string into ID
	idVal, err := parseID[ID](idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid ID: %v", err), http.StatusBadRequest)
		return
	}

	// call into the service with the correctly-typed ID
	err = s.Service.DeleteEntity(ctx, idVal)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, ErrNotFound) {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}

	http.Redirect(w, r, s.Service.Path, http.StatusSeeOther)
}
