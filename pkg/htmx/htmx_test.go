package htmx

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headerOnWire serializes the recorded response and returns the value of the
// named header as it is actually written to the connection, rather than the Go
// string held in the header map. Assertions that read the map instead would pass
// on raw UTF-8 that a browser decodes as ISO-8859-1.
func headerOnWire(t *testing.T, rec *httptest.ResponseRecorder, name string) string {
	t.Helper()
	res := rec.Result()
	defer func() {
		require.NoError(t, res.Body.Close())
	}()
	var buf bytes.Buffer
	require.NoError(t, res.Write(&buf))
	prefix := name + ": "
	for _, line := range strings.Split(buf.String(), "\r\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix)
		}
	}
	t.Fatalf("header %q not present on the wire:\n%s", name, buf.String())
	return ""
}

func requireASCII(t *testing.T, value string) {
	t.Helper()
	for i := 0; i < len(value); i++ {
		require.Lessf(t, value[i], byte(0x80),
			"byte %d of the header is 0x%02x, above US-ASCII: %q", i, value[i], value)
	}
}

type toastTrigger struct {
	name   string
	header string
	send   func(w http.ResponseWriter, variant ToastVariant, title, message string)
}

func toastTriggers() []toastTrigger {
	return []toastTrigger{
		{name: "TriggerToast", header: "Hx-Trigger", send: TriggerToast},
		{name: "TriggerToastAfterSwap", header: "Hx-Trigger-After-Swap", send: TriggerToastAfterSwap},
		{name: "TriggerToastAfterSettle", header: "Hx-Trigger-After-Settle", send: TriggerToastAfterSettle},
	}
}

// decodeToast parses the header value the way htmx does: JSON.parse of the whole
// value, then the "notify" event's detail.
func decodeToast(t *testing.T, value string) toastDetail {
	t.Helper()
	var payload map[string]toastDetail
	require.NoErrorf(t, json.Unmarshal([]byte(value), &payload),
		"header is not valid JSON, htmx would drop the event: %q", value)
	detail, ok := payload["notify"]
	require.Truef(t, ok, "no notify event in %q", value)
	return detail
}

// TestTriggerToast_NonASCIISurvivesAsASCII pins the reported production bug: a
// Cyrillic toast reached the operator as mojibake because the raw UTF-8 bytes in
// the header are decoded as ISO-8859-1 by the browser.
//
// Falsely green if it asserted only that json.Unmarshal returns the original
// text: raw UTF-8 unmarshals just fine in Go. The load-bearing assertion is that
// every byte written to the wire is below 0x80.
func TestTriggerToast_NonASCIISurvivesAsASCII(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		title   string
		message string
	}{
		{
			name:    "cyrillic",
			title:   "Ошибка",
			message: "Не удалось провести оплату…",
		},
		{
			name:    "non-BMP rune needs a surrogate pair",
			title:   "Готово 😀",
			message: "Платёж принят 🎉",
		},
		{
			name:    "mixed scripts",
			title:   "Xatolik",
			message: "Toʻlov amalga oshmadi — попробуйте ещё раз",
		},
	}

	for _, trigger := range toastTriggers() {
		for _, tt := range tests {
			t.Run(trigger.name+"/"+tt.name, func(t *testing.T) {
				t.Parallel()

				rec := httptest.NewRecorder()
				trigger.send(rec, ToastVariantError, tt.title, tt.message)

				value := headerOnWire(t, rec, trigger.header)
				requireASCII(t, value)

				detail := decodeToast(t, value)
				assert.Equal(t, ToastVariantError, detail.Variant)
				assert.Equal(t, tt.title, detail.Title)
				assert.Equal(t, tt.message, detail.Message)
			})
		}
	}
}

// TestTriggerToast_SpecialCharactersStayValidJSON covers the second defect: the
// detail used to be built with fmt.Sprintf, so a quote, a backslash or a control
// character produced JSON that htmx failed to parse and the toast never appeared.
//
// Falsely green if it only checked that the header contains the message text —
// a broken document can still contain it. The assertion that matters is that the
// value parses as JSON and the text comes back byte-identical.
func TestTriggerToast_SpecialCharactersStayValidJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
	}{
		{name: "double quote", message: `Полис "AB 1234567" не найден`},
		{name: "backslash", message: `path C:\policies\2026`},
		{name: "newline", message: "first line\nsecond line"},
		{name: "control character", message: "before\x01after"},
		{name: "all of them", message: "\"\\\n\x01 конец"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			TriggerToast(rec, ToastVariantWarning, `title with "quotes"`, tt.message)

			value := headerOnWire(t, rec, "Hx-Trigger")
			requireASCII(t, value)

			detail := decodeToast(t, value)
			assert.Equal(t, `title with "quotes"`, detail.Title)
			assert.Equal(t, tt.message, detail.Message)
		})
	}
}

// TestTriggerToast_ASCIIOnlyKeepsBehaviour guards existing consumers: an
// ASCII-only toast must still arrive as the same notify event with the same
// three fields.
//
// Falsely green if it compared the header byte-for-byte against the old
// fmt.Sprintf output — the separators changed (no space after the colon), while
// the parsed payload, which is all the client sees, did not.
func TestTriggerToast_ASCIIOnlyKeepsBehaviour(t *testing.T) {
	t.Parallel()

	for _, trigger := range toastTriggers() {
		t.Run(trigger.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			trigger.send(rec, ToastVariantSuccess, "Server Response", "This toast was triggered by an HTMX request!")

			value := headerOnWire(t, rec, trigger.header)
			requireASCII(t, value)

			detail := decodeToast(t, value)
			assert.Equal(t, toastDetail{
				Variant: ToastVariantSuccess,
				Title:   "Server Response",
				Message: "This toast was triggered by an HTMX request!",
			}, detail)
		})
	}
}

// TestTriggers_EmptyDetailUnchanged pins the detail-less form: htmx reads a value
// that is not JSON as a bare event name, so it must stay byte-for-byte identical.
//
// Falsely green if it asserted with Contains instead of Equal — a JSON-wrapped
// value contains the event name too, yet would change what htmx dispatches.
func TestTriggers_EmptyDetailUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		header string
		send   func(w http.ResponseWriter, event, detail string)
	}{
		{name: "SetTrigger", header: "Hx-Trigger", send: SetTrigger},
		{name: "TriggerAfterSwap", header: "Hx-Trigger-After-Swap", send: TriggerAfterSwap},
		{name: "TriggerAfterSettle", header: "Hx-Trigger-After-Settle", send: TriggerAfterSettle},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			tt.send(rec, "refresh-content", "")

			assert.Equal(t, "refresh-content", headerOnWire(t, rec, tt.header))
		})
	}
}

// TestSetTrigger_PreMarshalledDetailIsNormalised covers callers that hand
// SetTrigger a payload they marshalled themselves: encoding/json emits non-ASCII
// as raw UTF-8, so those triggers carried mojibake by the same route as toasts.
//
// Falsely green if it re-marshalled the detail before comparing — the fix must
// escape the caller's document without reparsing it, leaving key order and
// number formatting untouched.
func TestSetTrigger_PreMarshalledDetailIsNormalised(t *testing.T) {
	t.Parallel()

	type importSummary struct {
		Imported int    `json:"imported"`
		Reason   string `json:"reason"`
	}

	original := importSummary{Imported: 12, Reason: "Шаблон импортирован 🎉"}
	raw, err := json.Marshal(original)
	require.NoError(t, err)
	require.False(t, isASCII(string(raw)), "encoding/json is expected to emit raw UTF-8 here")

	rec := httptest.NewRecorder()
	SetTrigger(rec, "campaign:templates-imported", string(raw))

	value := headerOnWire(t, rec, "Hx-Trigger")
	requireASCII(t, value)

	var payload map[string]importSummary
	require.NoErrorf(t, json.Unmarshal([]byte(value), &payload), "header is not valid JSON: %q", value)
	assert.Equal(t, original, payload["campaign:templates-imported"])
}

// TestSetTrigger_EventNameIsEncoded covers the event half of the header, which
// used to be concatenated into the JSON unescaped.
//
// Falsely green if it used an event name without characters that need escaping —
// every ASCII event name in the repository parses fine either way.
func TestSetTrigger_EventNameIsEncoded(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	SetTrigger(rec, `weird"событие`, `{"ok":true}`)

	value := headerOnWire(t, rec, "Hx-Trigger")
	requireASCII(t, value)

	var payload map[string]map[string]bool
	require.NoErrorf(t, json.Unmarshal([]byte(value), &payload), "header is not valid JSON: %q", value)
	assert.Equal(t, map[string]map[string]bool{`weird"событие`: {"ok": true}}, payload)
}

// TestEscapeNonASCII_SurrogatePairs checks the escaping itself, including the
// two-escape form JSON requires outside the BMP.
//
// Falsely green if it only tested BMP runes: a single \uXXXX for U+1F600 is not
// representable and would silently truncate the rune.
func TestEscapeNonASCII_SurrogatePairs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "ascii untouched", input: `{"a":"b c"}`, expected: `{"a":"b c"}`},
		{name: "cyrillic", input: "Не", expected: `\u041d\u0435`},
		{name: "non-BMP becomes a surrogate pair", input: "😀", expected: `\ud83d\ude00`},
		{name: "boundary rune", input: "\u0080", expected: `\u0080`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, escapeNonASCII(tt.input))
		})
	}
}
