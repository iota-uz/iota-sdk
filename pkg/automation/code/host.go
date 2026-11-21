package code

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/google/uuid"
	kafig "github.com/iota-uz/kafig/kafig-go"
	"golang.org/x/net/http/httpguts"
)

// Bounds of one env.fetch call, checked in Go.
const (
	maxFetchURLBytes  = 16 << 10
	maxFetchHeaders   = 100
	maxFetchBodyBytes = 10 << 20
	maxDigestBytes    = 10 << 20
)

var fetchMethods = map[string]bool{
	http.MethodGet: true, http.MethodHead: true, http.MethodPost: true, http.MethodPut: true,
	http.MethodPatch: true, http.MethodDelete: true, http.MethodOptions: true,
}

// forbiddenFetchHeaders are set by the transport, never by the code.
var forbiddenFetchHeaders = map[string]bool{
	"host": true, "content-length": true, "transfer-encoding": true, "connection": true, "upgrade": true,
	"te": true, "trailer": true, "keep-alive": true, "proxy-connection": true, "proxy-authorization": true,
	"expect": true,
}

// reported is the one result the driver (or the code calling the host directly) reports.
type reported struct {
	Outcome *string `json:"outcome"`
	Output  *string `json:"output"`
	Error   *struct {
		Name    string `json:"name"`
		Message string `json:"message"`
		Stack   string `json:"stack"`
	} `json:"error"`
	Invalid *string `json:"invalid"`
}

// runState is the host side of one run: the router of host calls and everything they accumulate.
type runState struct {
	limits   Limits
	outcomes []string
	masker   Masker
	fetcher  Fetcher
	sink     LogSink
	done     func() // ends the run once the result is in

	fetches  atomic.Int64
	fetchSem chan struct{}
	blocked  atomic.Bool

	mu          sync.Mutex
	logs        []string
	logsDropped bool
	result      *reported
}

func newRunState(in RunInput, limits Limits, masker Masker, done func()) *runState {
	return &runState{
		limits:   limits,
		outcomes: in.Outcomes,
		masker:   masker,
		fetcher:  in.Fetcher,
		sink:     in.Logger,
		done:     done,
		fetchSem: make(chan struct{}, limits.MaxConcurrentFetches),
	}
}

// router publishes exactly the host calls of env; the result call is accepted once.
func (s *runState) router() *kafig.RPCRouter {
	return kafig.NewRPCRouter().
		WithSync("log", s.log).
		WithSync("uuid", s.uuid).
		WithSync("digest", s.digest).
		WithSync("__result", s.report).
		WithAsync("fetch", s.fetch)
}

func (s *runState) reported() *reported {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result
}

func (s *runState) report(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var r reported
	if err := json.Unmarshal(params, &r); err != nil {
		return nil, errors.New("invalid result")
	}
	s.mu.Lock()
	if s.result != nil {
		s.mu.Unlock()
		return nil, errors.New("the result was already reported")
	}
	s.result = &r
	s.mu.Unlock()
	s.done()
	return nil, nil
}

func (s *runState) log(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var args []string
	if err := json.Unmarshal(params, &args); err != nil {
		return nil, errors.New("log takes a list of strings")
	}
	line := s.masker.Mask(strings.Join(args, " "))
	if utf8.RuneCountInString(line) > s.limits.MaxLogLineChars {
		runes := []rune(line)
		line = string(runes[:s.limits.MaxLogLineChars]) + "…"
	}
	s.mu.Lock()
	switch {
	case len(s.logs) < s.limits.MaxLogLines:
		s.logs = append(s.logs, line)
	case !s.logsDropped:
		s.logsDropped = true
		line = fmt.Sprintf("… more log lines dropped: at most %d lines per run", s.limits.MaxLogLines)
		s.logs = append(s.logs, line)
	default:
		line = ""
	}
	s.mu.Unlock()
	if line != "" && s.sink != nil {
		s.sink.Log("info", line)
	}
	return nil, nil
}

func (s *runState) uuid(context.Context, json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(uuid.NewString())
}

func (s *runState) digest(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var p struct {
		Alg  string `json:"alg"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, errors.New("digest(alg, data): invalid arguments")
	}
	if !strings.EqualFold(p.Alg, "SHA-256") {
		return nil, errors.New(`digest: only "SHA-256" is supported`)
	}
	if len(p.Data) > maxDigestBytes {
		return nil, errors.New("digest: data is too long")
	}
	sum := sha256.Sum256([]byte(p.Data))
	return json.Marshal(hex.EncodeToString(sum[:]))
}

// fetchReply is what env.fetch receives: a response, or blocked / error, which it throws.
type fetchReply struct {
	Status     int               `json:"status,omitempty"`
	StatusText string            `json:"statusText,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       string            `json:"body,omitempty"`
	URL        string            `json:"url,omitempty"`
	Redirected bool              `json:"redirected,omitempty"`
	Blocked    string            `json:"blocked,omitempty"`
	Error      string            `json:"error,omitempty"`
}

func replyError(msg string) (json.RawMessage, error) { return json.Marshal(fetchReply{Error: msg}) }

func (s *runState) fetch(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	req, msg := parseFetch(params)
	if msg != "" {
		return replyError(msg)
	}
	if s.fetcher == nil {
		s.blocked.Store(true)
		return json.Marshal(fetchReply{Blocked: (&NetworkBlockedError{Reason: "disabled"}).Error()})
	}
	if n := s.fetches.Add(1); n > int64(s.limits.MaxFetches) {
		return replyError(fmt.Sprintf("at most %d fetches per run", s.limits.MaxFetches))
	}
	select {
	case s.fetchSem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-s.fetchSem }()

	resp, err := s.fetcher.Fetch(ctx, req)
	if err != nil {
		var blocked *NetworkBlockedError
		if errors.As(err, &blocked) {
			s.blocked.Store(true)
			return json.Marshal(fetchReply{Blocked: blocked.Error()})
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return replyError(err.Error())
	}
	headers := make(map[string]string, len(resp.Header))
	for k, vs := range resp.Header {
		headers[strings.ToLower(k)] = strings.Join(vs, ", ")
	}
	return json.Marshal(fetchReply{
		Status:     resp.Status,
		StatusText: resp.StatusText,
		Headers:    headers,
		Body:       strings.ToValidUTF8(string(resp.Body), "�"),
		URL:        resp.URL,
		Redirected: resp.Redirected,
	})
}

// parseFetch validates the parameters of a fetch host call; msg explains a refusal.
func parseFetch(params json.RawMessage) (req FetchRequest, msg string) {
	var p struct {
		URL      string            `json:"url"`
		Method   string            `json:"method"`
		Headers  map[string]string `json:"headers"`
		Body     *string           `json:"body"`
		Redirect string            `json:"redirect"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return req, "invalid fetch arguments"
	}
	if p.URL == "" || len(p.URL) > maxFetchURLBytes {
		return req, "invalid URL"
	}
	method := strings.ToUpper(p.Method)
	if method == "" {
		method = http.MethodGet
	}
	if !fetchMethods[method] {
		return req, "unsupported method " + p.Method
	}
	switch p.Redirect {
	case "":
		p.Redirect = "follow"
	case "follow", "manual", "error":
	default:
		return req, `redirect must be "follow", "manual" or "error"`
	}
	if len(p.Headers) > maxFetchHeaders {
		return req, "too many headers"
	}
	header := make(http.Header, len(p.Headers))
	for k, v := range p.Headers {
		if !httpguts.ValidHeaderFieldName(k) || !httpguts.ValidHeaderFieldValue(v) {
			return req, "invalid header " + k
		}
		lk := strings.ToLower(k)
		if forbiddenFetchHeaders[lk] || strings.HasPrefix(lk, "proxy-") || strings.HasPrefix(lk, "sec-") {
			return req, "header " + k + " cannot be set"
		}
		header.Add(k, v)
	}
	req = FetchRequest{URL: p.URL, Method: method, Header: header, Redirect: p.Redirect}
	if p.Body != nil {
		if len(*p.Body) > maxFetchBodyBytes {
			return FetchRequest{}, "request body is too long"
		}
		if method == http.MethodGet || method == http.MethodHead {
			return FetchRequest{}, "a GET or HEAD request cannot have a body"
		}
		req.Body = []byte(*p.Body)
	}
	return req, ""
}
