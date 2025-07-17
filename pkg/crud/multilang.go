package crud

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// =============================================================================
// MultiLang Implementation
// =============================================================================

// LangEntry represents a single language entry with code and text
type LangEntry struct {
	Code string `json:"code"`
	Text string `json:"text"`
}

// MultiLang represents a multilingual text object with dynamic language support
type MultiLang interface {
	JsonFieldData

	// Dynamic language access
	Get(langCode string) string
	Set(langCode, text string) MultiLang
	Add(langCode, text string) MultiLang
	Remove(langCode string) MultiLang

	// Bulk operations
	GetAll() []LangEntry
	SetAll(entries []LangEntry) MultiLang

	// Language management
	Languages() []string
	HasLanguage(langCode string) bool

	// Convenience methods for common languages
	Russian() string
	Uzbek() string
	English() string
	SetRussian(text string) MultiLang
	SetUzbek(text string) MultiLang
	SetEnglish(text string) MultiLang
}

// multiLang implements the MultiLang interface
type multiLang struct {
	entries []LangEntry
}

// NewMultiLang creates a new multilingual object
func NewMultiLang() MultiLang {
	return &multiLang{
		entries: []LangEntry{},
	}
}

// NewMultiLangFromEntries creates a multilingual object from entries
func NewMultiLangFromEntries(entries []LangEntry) MultiLang {
	return &multiLang{
		entries: append([]LangEntry{}, entries...),
	}
}

// NewMultiLangFromMap creates a multilingual object from a map (for backward compatibility)
func NewMultiLangFromMap(data map[string]string) MultiLang {
	entries := make([]LangEntry, 0, len(data))
	for code, text := range data {
		entries = append(entries, LangEntry{Code: code, Text: text})
	}
	// Sort for consistent order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Code < entries[j].Code
	})
	return NewMultiLangFromEntries(entries)
}

// Dynamic language access methods

func (m *multiLang) Get(langCode string) string {
	for _, entry := range m.entries {
		if entry.Code == langCode {
			return entry.Text
		}
	}
	return ""
}

func (m *multiLang) Set(langCode, text string) MultiLang {
	clone := m.clone()

	// Find existing entry and update it
	for i, entry := range clone.entries {
		if entry.Code == langCode {
			clone.entries[i].Text = text
			return clone
		}
	}

	// If not found, add new entry
	clone.entries = append(clone.entries, LangEntry{Code: langCode, Text: text})

	// Sort for consistent order
	sort.Slice(clone.entries, func(i, j int) bool {
		return clone.entries[i].Code < clone.entries[j].Code
	})

	return clone
}

func (m *multiLang) Add(langCode, text string) MultiLang {
	// Add is same as Set - will replace if exists or add if new
	return m.Set(langCode, text)
}

func (m *multiLang) Remove(langCode string) MultiLang {
	clone := m.clone()

	// Remove entry with matching code
	newEntries := make([]LangEntry, 0, len(clone.entries))
	for _, entry := range clone.entries {
		if entry.Code != langCode {
			newEntries = append(newEntries, entry)
		}
	}

	clone.entries = newEntries
	return clone
}

// Bulk operations

func (m *multiLang) GetAll() []LangEntry {
	// Return a copy to maintain immutability
	result := make([]LangEntry, len(m.entries))
	copy(result, m.entries)
	return result
}

func (m *multiLang) SetAll(entries []LangEntry) MultiLang {
	clone := m.clone()

	// Copy and sort entries
	clone.entries = make([]LangEntry, len(entries))
	copy(clone.entries, entries)

	sort.Slice(clone.entries, func(i, j int) bool {
		return clone.entries[i].Code < clone.entries[j].Code
	})

	return clone
}

// Language management

func (m *multiLang) Languages() []string {
	languages := make([]string, len(m.entries))
	for i, entry := range m.entries {
		languages[i] = entry.Code
	}
	return languages
}

func (m *multiLang) HasLanguage(langCode string) bool {
	for _, entry := range m.entries {
		if entry.Code == langCode {
			return true
		}
	}
	return false
}

// Convenience methods for common languages

func (m *multiLang) Russian() string {
	return m.Get("ru")
}

func (m *multiLang) Uzbek() string {
	return m.Get("uz")
}

func (m *multiLang) English() string {
	return m.Get("en")
}

func (m *multiLang) SetRussian(text string) MultiLang {
	return m.Set("ru", text)
}

func (m *multiLang) SetUzbek(text string) MultiLang {
	return m.Set("uz", text)
}

func (m *multiLang) SetEnglish(text string) MultiLang {
	return m.Set("en", text)
}

// JsonFieldData interface implementation

func (m *multiLang) ToJSON() (string, error) {
	data, err := json.Marshal(m.entries)
	return string(data), err
}

func (m *multiLang) FromJSON(jsonStr string) error {
	var entries []LangEntry
	if err := json.Unmarshal([]byte(jsonStr), &entries); err != nil {
		// Try to parse as map for backward compatibility
		var data map[string]string
		if mapErr := json.Unmarshal([]byte(jsonStr), &data); mapErr != nil {
			return fmt.Errorf("invalid JSON for multilang object: %w", err)
		}
		// Convert map to entries
		entries = make([]LangEntry, 0, len(data))
		for code, text := range data {
			entries = append(entries, LangEntry{Code: code, Text: text})
		}
		// Sort for consistent order
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Code < entries[j].Code
		})
	}

	m.entries = entries
	return nil
}

func (m *multiLang) ValidateData() error {
	// Basic validation - ensure no duplicate language codes
	langCodes := make(map[string]bool)
	for _, entry := range m.entries {
		if entry.Code == "" {
			return fmt.Errorf("language code cannot be empty")
		}
		if langCodes[entry.Code] {
			return fmt.Errorf("duplicate language code: %s", entry.Code)
		}
		langCodes[entry.Code] = true
	}
	return nil
}

// Helper functions

func (m *multiLang) clone() *multiLang {
	clone := &multiLang{
		entries: make([]LangEntry, len(m.entries)),
	}
	copy(clone.entries, m.entries)
	return clone
}

// MultiLangFromStrings creates a MultiLang object from individual language strings
func MultiLangFromStrings(russian, uzbek, english string) MultiLang {
	ml := NewMultiLang()
	if russian != "" {
		ml = ml.SetRussian(russian)
	}
	if uzbek != "" {
		ml = ml.SetUzbek(uzbek)
	}
	if english != "" {
		ml = ml.SetEnglish(english)
	}
	return ml
}

// MultiLangFromLangCodes creates a MultiLang object from language codes and texts
func MultiLangFromLangCodes(langMap map[string]string) MultiLang {
	entries := make([]LangEntry, 0, len(langMap))
	for code, text := range langMap {
		if strings.TrimSpace(text) != "" {
			entries = append(entries, LangEntry{Code: code, Text: text})
		}
	}
	return NewMultiLangFromEntries(entries)
}

// =============================================================================
// MultiLang JsonSchemaType Implementation
// =============================================================================

// multiLangSchemaType implements JsonSchemaTypeInterface for multilang schema
type multiLangSchemaType struct{}

func (m *multiLangSchemaType) CreateData() JsonFieldData {
	return NewMultiLang()
}

func (m *multiLangSchemaType) FormatJSON(data interface{}) (string, error) {
	if ml, ok := data.(MultiLang); ok {
		return ml.ToJSON()
	}

	// Fallback to standard JSON marshaling
	bytes, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(bytes), nil
}

func (m *multiLangSchemaType) ParseJSON(input string) (interface{}, error) {
	ml := NewMultiLang()
	if err := ml.FromJSON(input); err != nil {
		return nil, fmt.Errorf("failed to parse multilang JSON: %w", err)
	}
	return ml, nil
}

func (m *multiLangSchemaType) ValidateJSON(input string) error {
	ml := NewMultiLang()
	if err := ml.FromJSON(input); err != nil {
		return fmt.Errorf("invalid multilang JSON: %w", err)
	}
	return ml.ValidateData()
}

// Register the multilang schema type
func init() {
	RegisterJsonSchemaType("multilang", &multiLangSchemaType{})
}
