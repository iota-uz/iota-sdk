package kanban

import (
	"slices"

	"github.com/a-h/templ"
)

type Config[C Card] struct {
	ColumnChangeURL string
	CardChangeURL   string
	Board           Board[C]
}

func NewConfig[C Card](board Board[C], columnChangeURL, cardChangeURL string) *Config[C] {
	return &Config[C]{Board: board, ColumnChangeURL: columnChangeURL, CardChangeURL: cardChangeURL}
}

type Card interface {
	Key() string
	Component() templ.Component
}

type Column[C Card] interface {
	Key() string
	Title() templ.Component
	Cards() []C
}

type Board[C Card] interface {
	Key() string
	Title() string
	Columns() []Column[C]
}

type boardImpl[T Card] struct {
	key     string
	title   string
	columns []Column[T]
}

func (b *boardImpl[T]) Columns() []Column[T] {
	return b.columns
}

func (b *boardImpl[T]) Key() string {
	return b.key
}

func (b *boardImpl[T]) Title() string {
	return b.title
}

func NewBoard[T Card](key, title string, cols ...Column[T]) Board[T] {
	return &boardImpl[T]{
		key:     key,
		title:   title,
		columns: cols,
	}
}

type cardImpl struct {
	key       string
	component templ.Component
}

func (c *cardImpl) Component() templ.Component {
	return c.component
}

func (c *cardImpl) Key() string {
	return c.key
}

func NewCard(key string, component templ.Component) Card {
	return &cardImpl{key: key, component: component}
}

type columnImpl[T Card] struct {
	key   string
	title templ.Component
	cards []T
}

func (c *columnImpl[T]) Key() string {
	return c.key
}
func (c *columnImpl[T]) Title() templ.Component {
	return c.title
}

func (c *columnImpl[T]) Cards() []T {
	return c.cards
}

func (c *columnImpl[T]) AddCard(position int, card T) {
	c.cards = slices.Insert(c.cards, position, card)
}

func NewColumn[T Card](key string, title templ.Component, cards ...T) Column[T] {
	return &columnImpl[T]{
		key:   key,
		title: title,
		cards: cards,
	}
}
