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
	"time"

	"github.com/google/uuid"
	kafig "github.com/iota-uz/kafig/kafig-go"
	"golang.org/x/net/http/httpguts"
)

// Bounds of one env.fetch call, checked in Go.
const (
	maxFetchURLBytes = 16 << 10
	maxFetchHeaders  = 100
	// maxFetchHeaderBytes bounds the names and values of a request's headers together.
	maxFetchHeaderBytes = 64 << 10
	maxFetchBodyBytes   = 10 << 20
	// maxStagedFetchBytes bounds, in the exact bytes Go received, the staged requests of one run that wait
	// for their fetch together (and so one request). A request is staged by a synchronous call, admitted
	// here before it is kept; the async fetch call that kafig queues carries only the ticket.
	maxStagedFetchBytes = 16 << 20
	// maxFetchParamsBytes bounds the serialized request inside a staged payload.
	maxFetchParamsBytes = maxStagedFetchBytes
	maxDigestBytes      = 10 << 20
	// maxHeldBodyBytes bounds the response bodies one run's fetches keep in Go until the code reads them.
	maxHeldBodyBytes = 64 << 20
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
	timeout  time.Duration // the run's effective deadline from its start
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
	bodies      map[int64][]byte // response bodies not read yet, by id
	staged      map[int64][]byte // fetch requests staged and not fetched yet, by ticket
	stagedBytes int64
	nextTicket  int64
	heldBytes   int64
	nextBody    int64
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
		bodies:   map[int64][]byte{},
		staged:   map[int64][]byte{},
	}
}

// router publishes exactly the host calls of env; the result call is accepted once.
func (s *runState) router() *kafig.RPCRouter {
	return kafig.NewRPCRouter().
		WithSync("log", s.log).
		WithSync("uuid", s.uuid).
		WithSync("digest", s.digest).
		WithSync("body", s.body).
		WithSync("__result", s.report).
		WithSync("fetchStage", s.stage).
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
	var line string
	if err := json.Unmarshal(params, &line); err != nil {
		return nil, errors.New("log takes a string")
	}
	line = s.masker.Mask(line)
	if cut, ok := truncateRunes(line, s.limits.MaxLogLineChars); ok {
		line = cut + "…"
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

// truncateRunes cuts s to n runes; ok reports that it was longer.
func truncateRunes(s string, n int) (string, bool) {
	if len(s) <= n {
		return s, false
	}
	count := 0
	for i := range s {
		if count == n {
			return s[:i], true
		}
		count++
	}
	return s, false
}

// body hands the code a response body it reads and forgets it on the host.
func (s *runState) body(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	var id int64
	if err := json.Unmarshal(params, &id); err != nil {
		return nil, errors.New("body takes a response id")
	}
	s.mu.Lock()
	data, ok := s.bodies[id]
	delete(s.bodies, id)
	s.heldBytes -= int64(len(data))
	s.mu.Unlock()
	if !ok {
		return nil, errors.New("the body was already read")
	}
	return quoteJSON(strings.ToValidUTF8(string(data), "\uFFFD")), nil
}

// holdBody keeps a body until the code reads it; 0 for an empty body.
func (s *runState) holdBody(data []byte) (int64, bool) {
	if len(data) == 0 {
		return 0, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.heldBytes+int64(len(data)) > maxHeldBodyBytes {
		return 0, false
	}
	s.nextBody++
	s.bodies[s.nextBody] = data
	s.heldBytes += int64(len(data))
	return s.nextBody, true
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
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers"`
	BodyID     int64             `json:"bodyId"` // read with the body call; 0: empty
	URL        string            `json:"url"`
	Redirected bool              `json:"redirected"`
	Blocked    string            `json:"blocked,omitempty"`
	Error      string            `json:"error,omitempty"`
}

func replyError(msg string) (json.RawMessage, error) { return json.Marshal(fetchReply{Error: msg}) }

// stage admits one fetch request by the exact bytes Go received and returns its ticket. The bytes stay
// counted until the fetch that takes the ticket ends.
func (s *runState) stage(_ context.Context, params json.RawMessage) (json.RawMessage, error) {
	n := int64(len(params))
	if n > maxStagedFetchBytes {
		return nil, errors.New("request is too large")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stagedBytes+n > maxStagedFetchBytes {
		return nil, fmt.Errorf("more than %d MiB of requests wait for the network", maxStagedFetchBytes>>20)
	}
	s.nextTicket++
	s.staged[s.nextTicket] = params
	s.stagedBytes += n
	return json.Marshal(s.nextTicket)
}

// takeStaged returns a staged request and a func that stops counting its bytes.
func (s *runState) takeStaged(params json.RawMessage) ([]byte, func(), bool) {
	var ticket int64
	if err := json.Unmarshal(params, &ticket); err != nil {
		return nil, nil, false
	}
	s.mu.Lock()
	data, ok := s.staged[ticket]
	delete(s.staged, ticket)
	s.mu.Unlock()
	if !ok {
		return nil, nil, false
	}
	return data, func() {
		s.mu.Lock()
		s.stagedBytes -= int64(len(data))
		s.mu.Unlock()
	}, true
}

func (s *runState) fetch(ctx context.Context, params json.RawMessage) (json.RawMessage, error) {
	staged, done, ok := s.takeStaged(params)
	if !ok {
		return replyError("unknown fetch ticket")
	}
	defer done()
	var wire string // the staged payload is the request serialized once by the driver
	if err := json.Unmarshal(staged, &wire); err != nil {
		return replyError("invalid fetch arguments")
	}
	req, msg := parseFetch(json.RawMessage(wire))
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
	bodyID, ok := s.holdBody(resp.Body)
	if !ok {
		return replyError(fmt.Sprintf("unread response bodies exceed %d MiB: read the earlier responses first", maxHeldBodyBytes>>20))
	}
	return json.Marshal(fetchReply{
		Status:     resp.Status,
		StatusText: resp.StatusText,
		Headers:    headers,
		BodyID:     bodyID,
		URL:        resp.URL,
		Redirected: resp.Redirected,
	})
}

// parseFetch validates the parameters of a fetch host call; msg explains a refusal.
func parseFetch(params json.RawMessage) (FetchRequest, string) {
	var req FetchRequest
	var p struct {
		URL      string            `json:"url"`
		Method   string            `json:"method"`
		Headers  map[string]string `json:"headers"`
		Body     *string           `json:"body"`
		Redirect string            `json:"redirect"`
	}
	if len(params) > maxFetchParamsBytes {
		return req, "request is too large"
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
	headerBytes := 0
	for k, v := range p.Headers {
		headerBytes += len(k) + len(v)
		if headerBytes > maxFetchHeaderBytes {
			return req, "headers are too long"
		}
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
