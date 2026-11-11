package native_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/native"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

type line struct {
	SKU string `json:"sku"`
	Qty int    `json:"qty"`
}

type everything struct {
	Name    string          `json:"name" wire:"string,maxLength=40"`
	Count   int             `json:"count"`
	Big     int64           `json:"big,string" wire:"int64"`
	BigText string          `json:"bigText" wire:"int64"`
	Price   string          `json:"price" wire:"decimal,precision=18,scale=2"`
	Due     string          `json:"due" wire:"date"`
	At      time.Time       `json:"at"`
	AtText  string          `json:"atText" wire:"datetime"`
	ID      uuid.UUID       `json:"id"`
	Status  string          `json:"status" wire:"enum=draft|active"`
	Active  bool            `json:"active"`
	Extra   json.RawMessage `json:"extra"`
	Client  spi.ObjectRef   `json:"client" wire:"ref=crm.client"`
	Lines   []line          `json:"lines"`
	Tags    []string        `json:"tags" wire:"uuid"`
	Note    *string         `json:"note,omitempty"`
	Skipped string          `json:"-"`
	_       string
}

func TestTypeOfDerivesTheWireTypeFromTags(t *testing.T) {
	got, err := native.TypeOf[everything]()
	require.NoError(t, err)
	str := manifest.Type{Kind: manifest.KindString}
	want := manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
		{Name: "name", Type: manifest.Type{Kind: manifest.KindString, MaxLength: 40}},
		{Name: "count", Type: manifest.Type{Kind: manifest.KindInt}},
		{Name: "big", Type: manifest.Type{Kind: manifest.KindInt64}},
		{Name: "bigText", Type: manifest.Type{Kind: manifest.KindInt64}},
		{Name: "price", Type: manifest.Type{Kind: manifest.KindDecimal, Precision: 18, Scale: 2}},
		{Name: "due", Type: manifest.Type{Kind: manifest.KindDate}},
		{Name: "at", Type: manifest.Type{Kind: manifest.KindDateTime}},
		{Name: "atText", Type: manifest.Type{Kind: manifest.KindDateTime}},
		{Name: "id", Type: manifest.Type{Kind: manifest.KindUUID}},
		{Name: "status", Type: manifest.Type{Kind: manifest.KindEnum, Values: []string{"draft", "active"}}},
		{Name: "active", Type: manifest.Type{Kind: manifest.KindBool}},
		{Name: "extra", Type: manifest.Type{Kind: manifest.KindJSON}},
		{Name: "client", Type: manifest.Type{Kind: manifest.KindRef, Entity: "crm.client"}},
		{Name: "lines", Type: manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindObject, Properties: []manifest.Property{
			{Name: "sku", Type: str}, {Name: "qty", Type: manifest.Type{Kind: manifest.KindInt}},
		}}}},
		{Name: "tags", Type: manifest.Type{Kind: manifest.KindArray, Items: &manifest.Type{Kind: manifest.KindUUID}}},
		{Name: "note", Type: manifest.Type{Kind: manifest.KindString, Optional: true}},
	}}
	require.Equal(t, want, got)

	// What Go marshals is what the type accepts.
	v := everything{Name: "n", Big: 9007199254740993, BigText: "1", Price: "1.50", Due: "2026-01-02", At: time.Now(), AtText: "2026-01-02T03:04:05Z",
		ID: uuid.New(), Status: "draft", Extra: json.RawMessage(`{"a":1}`), Client: spi.ObjectRef{Entity: "crm.client", Key: "1"},
		Lines: []line{{SKU: "a", Qty: 1}}, Tags: []string{uuid.NewString()}}
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, manifest.ValidateValue(got, raw))
}

func TestTypeOfFailsLoudlyOnWhatTheWireCannotCarry(t *testing.T) {
	check := func(name string, fn func() (manifest.Type, error), msg string) {
		t.Run(name, func(t *testing.T) {
			_, err := fn()
			require.ErrorContains(t, err, msg)
		})
	}
	check("int64 without string", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			N int64 `json:"n"`
		}]()
	}, "travels as a decimal string")
	check("int64 string without wire", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			N int64 `json:"n,string"`
		}]()
	}, "wire:\"int64\"")
	check("float", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			F float64 `json:"f"`
		}]()
	}, "has no wire type")
	check("map", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			M map[string]string `json:"m"`
		}]()
	}, "has no wire type")
	check("missing json tag", func() (manifest.Type, error) {
		return native.TypeOf[struct{ A string }]()
	}, "needs a json tag")
	check("pointer without omitempty", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			P *string `json:"p"`
		}]()
	}, "needs `omitempty`")
	check("omitempty on a required field", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			S string `json:"s,omitempty"`
		}]()
	}, "make it a pointer")
	check("ref without entity", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			R spi.ObjectRef `json:"r"`
		}]()
	}, "ref=<entity id>")
	check("unknown wire", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			S string `json:"s" wire:"money"`
		}]()
	}, "unknown wire type")
	check("wire on a bool", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			B bool `json:"b" wire:"date"`
		}]()
	}, "does not apply")
	check("date on time.Time", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			T time.Time `json:"t" wire:"date"`
		}]()
	}, "needs a string field")
	check("bytes", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			B []byte `json:"b"`
		}]()
	}, "not supported")
	type node struct {
		Next []node `json:"next"`
	}
	check("recursive", func() (manifest.Type, error) { return native.TypeOf[node]() }, "recursive")
	check("embedded", func() (manifest.Type, error) {
		return native.TypeOf[struct {
			line
		}]()
	}, "embedded")
}

func TestDefinitionsPublishTheirContract(t *testing.T) {
	_, err := native.NewOperation[struct {
		F float64 `json:"f"`
	}, line]("acme.x.op", native.Spec{}, nil)
	require.ErrorContains(t, err, "acme.x.op input")
	_, err = native.NewOperation[line, line]("Bad.ID", native.Spec{}, nil)
	require.ErrorContains(t, err, "invalid")

	op := native.Operation[line, line]("acme.x.op", native.Spec{Permission: "X.Write", Errors: []string{"acme.x.gone"}, Description: "d"}, nil)
	cmd := native.AtomicCommand[line, line]("acme.x.atomic", native.Spec{}, nil)
	r := op.Resource()
	require.Equal(t, manifest.ResOperation, r.Kind)
	require.Empty(t, r.Permission, "an RBAC name is not a catalog resource; the host enforces it from the definition")
	require.Equal(t, "X.Write", op.Permission())
	require.Equal(t, []string{"acme.x.gone"}, r.Errors)
	require.Equal(t, manifest.ContractHashOf(r), r.ContractHash)
	require.False(t, op.HasDefault())
	require.True(t, cmd.Resource().Atomic)

	set := native.NewSet()
	require.NoError(t, set.Register(op, cmd, op))
	require.Error(t, set.Register(native.Operation[line, line]("acme.x.op", native.Spec{}, nil)), "one definition per ID")
	require.Equal(t, []string{"acme.x.atomic", "acme.x.op"}, []string{set.Resources()[0].ID, set.Resources()[1].ID})
}
