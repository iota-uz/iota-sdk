package drill

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
)

const (
	QueryTrail       = "_lens_drill"
	QueryPageTitle   = "_lens_page_title"
	QueryScopeLabel  = "_lens_scope_label"
	QueryScopeValue  = "_lens_scope_value"
	QueryDestination = "_lens_destination"
)

type Crumb struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	ScopeLabel  string `json:"scopeLabel,omitempty"`
	ScopeValue  string `json:"scopeValue,omitempty"`
	Destination string `json:"destination,omitempty"`
}

func (c Crumb) DisplayLabel() string {
	if strings.TrimSpace(c.ScopeValue) != "" {
		return c.ScopeValue
	}
	return c.Title
}

type State struct {
	Trail       []Crumb
	Current     Crumb
	nextTrail   []Crumb
	encodedNext string
}

func Parse(path string, values url.Values, currentTitle string) *State {
	state := &State{
		Trail: decodeTrail(values.Get(QueryTrail)),
		Current: Crumb{
			URL:         buildCurrentURL(path, values),
			Title:       firstNonEmpty(values.Get(QueryPageTitle), currentTitle),
			ScopeLabel:  values.Get(QueryScopeLabel),
			ScopeValue:  values.Get(QueryScopeValue),
			Destination: values.Get(QueryDestination),
		},
	}
	state.nextTrail = appendCurrentCrumb(state.Trail, state.Current)
	state.encodedNext = encodeTrail(state.nextTrail)
	return state
}

func (s *State) HasNavigation() bool {
	if s == nil {
		return false
	}
	return len(s.Trail) > 0 || strings.TrimSpace(s.Current.ScopeValue) != ""
}

func (s *State) Up() (Crumb, bool) {
	if s == nil || len(s.Trail) == 0 {
		return Crumb{}, false
	}
	return s.Trail[len(s.Trail)-1], true
}

func (s *State) NextTrailEncoded() string {
	if s == nil {
		return ""
	}
	return s.encodedNext
}

func Strip(values url.Values) url.Values {
	clean := url.Values{}
	for key, items := range values {
		if isReservedKey(key) {
			continue
		}
		for _, item := range items {
			clean.Add(key, item)
		}
	}
	return clean
}

func HiddenFields(values url.Values) map[string]string {
	fields := map[string]string{}
	for _, key := range []string{QueryTrail, QueryPageTitle, QueryScopeLabel, QueryScopeValue, QueryDestination} {
		if value := strings.TrimSpace(values.Get(key)); value != "" {
			fields[key] = value
		}
	}
	return fields
}

func isReservedKey(key string) bool {
	switch key {
	case QueryTrail, QueryPageTitle, QueryScopeLabel, QueryScopeValue, QueryDestination:
		return true
	default:
		return false
	}
}

func appendCurrentCrumb(trail []Crumb, current Crumb) []Crumb {
	if strings.TrimSpace(current.URL) == "" || strings.TrimSpace(current.Title) == "" {
		return append([]Crumb(nil), trail...)
	}
	next := append([]Crumb(nil), trail...)
	if len(next) > 0 && next[len(next)-1].URL == current.URL {
		return next
	}
	return append(next, current)
}

func buildCurrentURL(path string, values url.Values) string {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		trimmedPath = "/"
	}
	query := values.Encode()
	if query == "" {
		return trimmedPath
	}
	return trimmedPath + "?" + query
}

func encodeTrail(trail []Crumb) string {
	if len(trail) == 0 {
		return ""
	}
	payload, err := json.Marshal(trail)
	if err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeTrail(raw string) []Crumb {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil
	}
	var trail []Crumb
	if err := json.Unmarshal(payload, &trail); err != nil {
		return nil
	}
	return trail
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
