package context

// BlockQuery provides a DSL for filtering blocks in the graph.
type BlockQuery interface {
	// Matches returns true if the block matches this query.
	Matches(block ContextBlock) bool

	// And combines this query with another using logical AND.
	And(other BlockQuery) BlockQuery

	// Or combines this query with another using logical OR.
	Or(other BlockQuery) BlockQuery

	// Not negates this query.
	Not() BlockQuery
}

// kindQuery filters by block kind.
type kindQuery struct {
	kind BlockKind
}

// Kind creates a query that matches blocks of the specified kind.
func Kind(kind BlockKind) BlockQuery {
	return &kindQuery{kind: kind}
}

func (q *kindQuery) Matches(block ContextBlock) bool {
	return block.Meta.Kind == q.kind
}

func (q *kindQuery) And(other BlockQuery) BlockQuery {
	return &andQuery{left: q, right: other}
}

func (q *kindQuery) Or(other BlockQuery) BlockQuery {
	return &orQuery{left: q, right: other}
}

func (q *kindQuery) Not() BlockQuery {
	return &notQuery{query: q}
}

// sensitivityQuery filters by sensitivity level.
type sensitivityQuery struct {
	level SensitivityLevel
}

// Sensitivity creates a query that matches blocks of the specified sensitivity.
func Sensitivity(level SensitivityLevel) BlockQuery {
	return &sensitivityQuery{level: level}
}

func (q *sensitivityQuery) Matches(block ContextBlock) bool {
	return block.Meta.Sensitivity == q.level
}

func (q *sensitivityQuery) And(other BlockQuery) BlockQuery {
	return &andQuery{left: q, right: other}
}

func (q *sensitivityQuery) Or(other BlockQuery) BlockQuery {
	return &orQuery{left: q, right: other}
}

func (q *sensitivityQuery) Not() BlockQuery {
	return &notQuery{query: q}
}

// hasTagQuery filters by tag presence.
type hasTagQuery struct {
	tag string
}

// HasTag creates a query that matches blocks containing the specified tag.
func HasTag(tag string) BlockQuery {
	return &hasTagQuery{tag: tag}
}

func (q *hasTagQuery) Matches(block ContextBlock) bool {
	for _, t := range block.Meta.Tags {
		if t == q.tag {
			return true
		}
	}
	return false
}

func (q *hasTagQuery) And(other BlockQuery) BlockQuery {
	return &andQuery{left: q, right: other}
}

func (q *hasTagQuery) Or(other BlockQuery) BlockQuery {
	return &orQuery{left: q, right: other}
}

func (q *hasTagQuery) Not() BlockQuery {
	return &notQuery{query: q}
}

// sourceQuery filters by source identifier.
type sourceQuery struct {
	source string
}

// Source creates a query that matches blocks with the specified source.
func Source(source string) BlockQuery {
	return &sourceQuery{source: source}
}

func (q *sourceQuery) Matches(block ContextBlock) bool {
	return block.Meta.Source == q.source
}

func (q *sourceQuery) And(other BlockQuery) BlockQuery {
	return &andQuery{left: q, right: other}
}

func (q *sourceQuery) Or(other BlockQuery) BlockQuery {
	return &orQuery{left: q, right: other}
}

func (q *sourceQuery) Not() BlockQuery {
	return &notQuery{query: q}
}

// andQuery combines two queries with logical AND.
type andQuery struct {
	left  BlockQuery
	right BlockQuery
}

func (q *andQuery) Matches(block ContextBlock) bool {
	return q.left.Matches(block) && q.right.Matches(block)
}

func (q *andQuery) And(other BlockQuery) BlockQuery {
	return &andQuery{left: q, right: other}
}

func (q *andQuery) Or(other BlockQuery) BlockQuery {
	return &orQuery{left: q, right: other}
}

func (q *andQuery) Not() BlockQuery {
	return &notQuery{query: q}
}

// orQuery combines two queries with logical OR.
type orQuery struct {
	left  BlockQuery
	right BlockQuery
}

func (q *orQuery) Matches(block ContextBlock) bool {
	return q.left.Matches(block) || q.right.Matches(block)
}

func (q *orQuery) And(other BlockQuery) BlockQuery {
	return &andQuery{left: q, right: other}
}

func (q *orQuery) Or(other BlockQuery) BlockQuery {
	return &orQuery{left: q, right: other}
}

func (q *orQuery) Not() BlockQuery {
	return &notQuery{query: q}
}

// notQuery negates a query.
type notQuery struct {
	query BlockQuery
}

func (q *notQuery) Matches(block ContextBlock) bool {
	return !q.query.Matches(block)
}

func (q *notQuery) And(other BlockQuery) BlockQuery {
	return &andQuery{left: q, right: other}
}

func (q *notQuery) Or(other BlockQuery) BlockQuery {
	return &orQuery{left: q, right: other}
}

func (q *notQuery) Not() BlockQuery {
	return q.query // Double negation
}

// AllQuery matches all blocks.
type AllQuery struct{}

// All returns a query that matches all blocks.
func All() BlockQuery {
	return &AllQuery{}
}

func (q *AllQuery) Matches(block ContextBlock) bool {
	return true
}

func (q *AllQuery) And(other BlockQuery) BlockQuery {
	return other
}

func (q *AllQuery) Or(other BlockQuery) BlockQuery {
	return q
}

func (q *AllQuery) Not() BlockQuery {
	return &NoneQuery{}
}

// NoneQuery matches no blocks.
type NoneQuery struct{}

// None returns a query that matches no blocks.
func None() BlockQuery {
	return &NoneQuery{}
}

func (q *NoneQuery) Matches(block ContextBlock) bool {
	return false
}

func (q *NoneQuery) And(other BlockQuery) BlockQuery {
	return q
}

func (q *NoneQuery) Or(other BlockQuery) BlockQuery {
	return other
}

func (q *NoneQuery) Not() BlockQuery {
	return &AllQuery{}
}
