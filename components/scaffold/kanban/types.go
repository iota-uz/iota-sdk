package kanban

import (
	"context"
	"net/url"
	"slices"
	"strconv"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/pkg/composables"
)

type Config[C Card] struct {
	ColumnChangeURL string
	CardChangeURL   string
	CardLoadURL     string
	ColumnLoads     map[string]*ColumnLoadState
	Board           Board[C]
}

func NewConfig[C Card](board Board[C], columnChangeURL, cardChangeURL string) *Config[C] {
	return &Config[C]{
		Board:           board,
		ColumnChangeURL: columnChangeURL,
		CardChangeURL:   cardChangeURL,
		ColumnLoads:     make(map[string]*ColumnLoadState),
	}
}

const QueryParamColumn = "kanbanColumn"

const (
	queryParamPage  = "page"
	queryParamLimit = "limit"
)

type ColumnLoadState struct {
	NextPage int
	PerPage  int
}

func (c *Config[C]) WithCardLoadURL(url string) *Config[C] {
	c.CardLoadURL = url
	return c
}

func (c *Config[C]) WithColumnLoad(columnKey string, state *ColumnLoadState) *Config[C] {
	if c.ColumnLoads == nil {
		c.ColumnLoads = make(map[string]*ColumnLoadState)
	}

	if state == nil || state.NextPage < 1 || state.PerPage < 1 {
		delete(c.ColumnLoads, columnKey)
		return c
	}

	c.ColumnLoads[columnKey] = state
	return c
}

func (c *Config[C]) WithColumnLoads(loads map[string]*ColumnLoadState) *Config[C] {
	if c.ColumnLoads == nil {
		c.ColumnLoads = make(map[string]*ColumnLoadState, len(loads))
	}

	for key, state := range loads {
		c.WithColumnLoad(key, state)
	}

	return c
}

func (c *Config[C]) ColumnLoad(columnKey string) *ColumnLoadState {
	if c == nil || c.ColumnLoads == nil {
		return nil
	}

	state, ok := c.ColumnLoads[columnKey]
	if !ok || state == nil || state.NextPage < 1 || state.PerPage < 1 {
		return nil
	}

	return state
}

func CurrentQueryParams(ctx context.Context) url.Values {
	params, _ := composables.UseParams(ctx)
	currentParams := url.Values{}
	if params != nil && params.Request != nil {
		currentParams = params.Request.URL.Query()
	}

	return currentParams
}

func ColumnCardsTargetID(columnKey string) string {
	return "kanban-column-cards-" + columnKey
}

func ColumnSpinnerID(columnKey string) string {
	return "kanban-column-spinner-" + columnKey
}

func nextColumnChunkURL(baseURL, columnKey string, state *ColumnLoadState, currentParams url.Values) string {
	if state == nil {
		return baseURL
	}

	params := url.Values{}
	for key, values := range currentParams {
		for _, value := range values {
			params.Add(key, value)
		}
	}

	params.Set(QueryParamColumn, columnKey)
	params.Set(queryParamPage, strconv.Itoa(state.NextPage))
	params.Set(queryParamLimit, strconv.Itoa(state.PerPage))

	return baseURL + "?" + params.Encode()
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
