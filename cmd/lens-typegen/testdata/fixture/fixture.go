package fixture

import "time"

const ContractVersion = "3.2.1"

type FixtureKind string

const (
	FixtureKindZeta  FixtureKind = "zeta"
	FixtureKindAlpha FixtureKind = "alpha"
)

type FixtureErrorCode string

const (
	FixtureErrorNotFound    FixtureErrorCode = "NOT_FOUND"
	FixtureErrorUnavailable FixtureErrorCode = "UNAVAILABLE"
)

type FixtureDocument struct {
	Nested    Nested            `json:"nested"`
	Items     []Nested          `json:"items"`
	Lookup    map[string]Nested `json:"lookup"`
	Kind      FixtureKind       `json:"kind"`
	Maybe     *Nested           `json:"maybe,omitempty"`
	CreatedAt time.Time         `json:"createdAt"`
	Count     int               `json:"count"`
	Payload   any               `json:"payload"`
}

type Nested struct {
	Name     string `json:"name"`
	Optional string `json:"optional,omitempty"`
}
