package codex

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type RPCRequest struct {
	JSONRPC string `json:"jsonrpc,omitempty"`
	ID      uint64 `json:"id,omitempty"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type RPCResponse struct {
	JSONRPC string          `json:"jsonrpc,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type InitializeParams struct {
	ClientInfo   ClientInfo              `json:"clientInfo"`
	Capabilities *InitializeCapabilities `json:"capabilities,omitempty"`
}

type ClientInfo struct {
	Name    string  `json:"name"`
	Title   *string `json:"title,omitempty"`
	Version string  `json:"version"`
}

type InitializeCapabilities struct {
	ExperimentalAPI bool `json:"experimentalApi"`
}

type InitializeResult struct {
	UserAgent      string `json:"userAgent"`
	CodexHome      string `json:"codexHome"`
	PlatformFamily string `json:"platformFamily"`
	PlatformOS     string `json:"platformOs"`
}

type Client struct {
	Reader  io.Reader
	Writer  io.Writer
	MaxWait time.Duration

	stateMu          sync.Mutex
	nextID           uint64
	handshakeStarted bool
	handshaken       bool
	capabilities     map[string]struct{}
	frames           *FrameReader
	pending          map[uint64]chan callResult
	inbound          chan json.RawMessage
	done             chan struct{}
	readerOnce       sync.Once
	writerOnce       sync.Once
	failOnce         sync.Once
	transportOnce    sync.Once
	readerErr        error
	writes           chan writeRequest
}

type callResult struct {
	response RPCResponse
	err      error
}

type writeRequest struct {
	ctx    context.Context
	value  any
	result chan error
}

func NewClient(reader io.Reader, writer io.Writer) *Client {
	return &Client{Reader: reader, Writer: writer, MaxWait: 5 * time.Second, nextID: 1,
		capabilities: make(map[string]struct{}), frames: NewFrameReader(reader),
		pending: make(map[uint64]chan callResult), inbound: make(chan json.RawMessage, 64), done: make(chan struct{}),
		writes: make(chan writeRequest, 32)}
}

// ConfigureMethods installs the exact schema-derived allowlist before the
// initialize handshake. The current app-server initialize response does not
// negotiate or return a method list.
func (c *Client) ConfigureMethods(methods []string) error {
	if c == nil {
		return domain.NewError(domain.CodeInvalidArgument, "codex protocol client is unavailable")
	}
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.handshakeStarted || c.handshaken {
		return domain.NewError(domain.CodeConflict, "codex method allowlist is already frozen")
	}
	configured := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		if method == "" || len(method) > MaxMethodBytes {
			return domain.NewError(domain.CodeInvalidArgument, "codex method allowlist is invalid")
		}
		configured[method] = struct{}{}
	}
	c.capabilities = configured
	return nil
}

func (c *Client) Handshake(ctx context.Context, params InitializeParams) (InitializeResult, error) {
	if c == nil || c.Reader == nil || c.Writer == nil || ctx == nil {
		return InitializeResult{}, domain.NewError(domain.CodeInvalidArgument, "codex protocol client is incomplete")
	}
	if strings.TrimSpace(params.ClientInfo.Name) == "" || strings.TrimSpace(params.ClientInfo.Version) == "" || params.Capabilities == nil || params.Capabilities.ExperimentalAPI {
		return InitializeResult{}, domain.NewError(domain.CodeInvalidArgument, "codex initialize parameters are invalid")
	}
	c.stateMu.Lock()
	if c.handshakeStarted || c.handshaken {
		c.stateMu.Unlock()
		return InitializeResult{}, domain.NewError(domain.CodeConflict, "codex protocol handshake already completed")
	}
	c.handshakeStarted = true
	c.stateMu.Unlock()
	var raw json.RawMessage
	if err := c.call(ctx, MethodInitialize, params, &raw); err != nil {
		return InitializeResult{}, err
	}
	var result InitializeResult
	if len(raw) > 0 {
		if err := DecodeObject(raw, &result); err != nil {
			return InitializeResult{}, err
		}
	}
	if strings.TrimSpace(result.UserAgent) == "" || strings.TrimSpace(result.CodexHome) == "" ||
		strings.TrimSpace(result.PlatformFamily) == "" || strings.TrimSpace(result.PlatformOS) == "" ||
		len(result.UserAgent) > 512 || len(result.CodexHome) > 4096 || len(result.PlatformFamily) > 64 || len(result.PlatformOS) > 64 {
		return InitializeResult{}, domain.NewError(domain.CodeProviderProtocolError, "codex initialize result is invalid")
	}
	if err := c.write(ctx, RPCRequest{JSONRPC: "2.0", Method: MethodInitialized}); err != nil {
		return InitializeResult{}, err
	}
	c.stateMu.Lock()
	c.handshaken = true
	c.stateMu.Unlock()
	return result, nil
}

// RespondServerRequest writes a JSON-RPC result for a Provider-initiated
// request. The raw request ID is preserved but never interpreted as authority.
func (c *Client) RespondServerRequest(ctx context.Context, id json.RawMessage, result any) error {
	if c == nil || c.Writer == nil || ctx == nil || len(id) == 0 || len(id) > 256 {
		return domain.NewError(domain.CodeInvalidArgument, "codex server response is incomplete")
	}
	c.stateMu.Lock()
	if !c.handshaken {
		c.stateMu.Unlock()
		return domain.NewError(domain.CodeProviderProtocolError, "codex initialize handshake is required")
	}
	c.stateMu.Unlock()
	return c.write(ctx, RPCResponse{JSONRPC: "2.0", ID: id, Result: mustRawJSON(result)})
}

func mustRawJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	if c == nil || c.Reader == nil || c.Writer == nil || ctx == nil || method == "" {
		return domain.NewError(domain.CodeInvalidArgument, "codex protocol call is incomplete")
	}
	c.stateMu.Lock()
	if !c.handshaken {
		c.stateMu.Unlock()
		return domain.NewError(domain.CodeProviderProtocolError, "codex initialize handshake is required")
	}
	if method != MethodInitialize && method != MethodInitialized {
		if _, ok := c.capabilities[method]; !ok {
			c.stateMu.Unlock()
			return domain.NewError(domain.CodeProviderVersionUnsupported, "codex method is not enabled")
		}
	}
	c.stateMu.Unlock()
	return c.call(ctx, method, params, result)
}

// ReadInbound returns Provider-initiated notifications or server requests in
// arrival order. Responses observed while waiting on a client call are never
// mistaken for events.
func (c *Client) ReadInbound(ctx context.Context) (json.RawMessage, error) {
	if c == nil || ctx == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "codex inbound read is incomplete")
	}
	timer := time.NewTimer(c.waitDuration())
	defer timer.Stop()
	select {
	case frame := <-c.inbound:
		return append(json.RawMessage(nil), frame...), nil
	case <-c.done:
		return nil, c.failure()
	case <-ctx.Done():
		return nil, domain.NewError(domain.CodeDeadlineExceeded, "codex inbound read was cancelled")
	case <-timer.C:
		return nil, domain.NewError(domain.CodeDeadlineExceeded, "codex inbound read timed out")
	}
}

func (c *Client) call(ctx context.Context, method string, params, result any) error {
	c.stateMu.Lock()
	select {
	case <-c.done:
		c.stateMu.Unlock()
		return c.failure()
	default:
	}
	id := c.nextID
	c.nextID++
	responseCh := make(chan callResult, 1)
	c.pending[id] = responseCh
	c.stateMu.Unlock()
	c.readerOnce.Do(func() { go c.readerLoop() })
	if err := c.write(ctx, RPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		c.removePending(id)
		c.fail(err)
		return err
	}
	timer := time.NewTimer(c.waitDuration())
	defer timer.Stop()
	select {
	case received := <-responseCh:
		if received.err != nil {
			return received.err
		}
		if received.response.Error != nil {
			return domain.NewError(domain.CodeProviderProtocolError, "codex request was rejected")
		}
		if result != nil && len(received.response.Result) > 0 {
			encoded := received.response.Result
			if raw, ok := result.(*json.RawMessage); ok {
				*raw = append((*raw)[:0], encoded...)
				return nil
			}
			return DecodeObject(encoded, result)
		}
		return nil
	case <-c.done:
		c.removePending(id)
		return c.failure()
	case <-ctx.Done():
		c.removePending(id)
		return domain.NewError(domain.CodeDeadlineExceeded, "codex protocol call was cancelled")
	case <-timer.C:
		c.removePending(id)
		return domain.NewError(domain.CodeDeadlineExceeded, "codex protocol call timed out")
	}
}

func (c *Client) readerLoop() {
	for {
		frame, err := c.frames.Read()
		if err != nil {
			c.fail(err)
			return
		}
		var envelope struct {
			JSONRPC string          `json:"jsonrpc,omitempty"`
			ID      json.RawMessage `json:"id,omitempty"`
			Method  string          `json:"method,omitempty"`
			Params  json.RawMessage `json:"params,omitempty"`
			Result  json.RawMessage `json:"result,omitempty"`
			Error   *RPCError       `json:"error,omitempty"`
		}
		if err := DecodeObject(frame, &envelope); err != nil {
			c.fail(err)
			return
		}
		if envelope.Method != "" {
			select {
			case c.inbound <- append(json.RawMessage(nil), frame...):
			default:
				c.fail(domain.NewError(domain.CodeResourceExhausted, "codex inbound event queue is full"))
				return
			}
			continue
		}
		id, err := strconv.ParseUint(string(envelope.ID), 10, 64)
		if err != nil || id == 0 {
			c.fail(domain.NewError(domain.CodeProviderProtocolError, "codex response id is invalid"))
			return
		}
		c.stateMu.Lock()
		waiter := c.pending[id]
		delete(c.pending, id)
		c.stateMu.Unlock()
		if waiter == nil {
			c.fail(domain.NewError(domain.CodeProviderProtocolError, "codex response id is unknown"))
			return
		}
		waiter <- callResult{response: RPCResponse{JSONRPC: envelope.JSONRPC, ID: envelope.ID, Result: envelope.Result, Error: envelope.Error}}
	}
}

// write hands every frame to one client-owned writer goroutine. A write that
// exceeds the same bounded protocol deadline fails the client and closes its
// transports, which is the only portable way to unblock an io.Writer without
// introducing a second concurrent writer.
func (c *Client) write(ctx context.Context, value any) error {
	if ctx == nil {
		return domain.NewError(domain.CodeInvalidArgument, "codex protocol write context is missing")
	}
	writeCtx, cancel := context.WithTimeout(ctx, c.waitDuration())
	defer cancel()
	c.writerOnce.Do(func() { go c.writerLoop() })
	request := writeRequest{ctx: writeCtx, value: value, result: make(chan error, 1)}
	select {
	case c.writes <- request:
	case <-c.done:
		return c.failure()
	case <-writeCtx.Done():
		err := domain.NewError(domain.CodeDeadlineExceeded, "codex protocol write timed out")
		c.fail(err)
		return err
	}
	select {
	case err := <-request.result:
		return err
	case <-c.done:
		return c.failure()
	case <-writeCtx.Done():
		err := domain.NewError(domain.CodeDeadlineExceeded, "codex protocol write timed out")
		c.fail(err)
		return err
	}
}

func (c *Client) writerLoop() {
	for {
		select {
		case <-c.done:
			return
		case request := <-c.writes:
			select {
			case <-request.ctx.Done():
				request.result <- domain.NewError(domain.CodeDeadlineExceeded, "codex protocol write timed out")
				continue
			default:
			}
			err := WriteFrame(c.Writer, request.value)
			request.result <- err
			if err != nil {
				c.fail(err)
				return
			}
		}
	}
}

func (c *Client) removePending(id uint64) {
	c.stateMu.Lock()
	delete(c.pending, id)
	c.stateMu.Unlock()
}

func (c *Client) fail(err error) {
	if err == nil {
		err = domain.NewError(domain.CodeProviderFailed, "codex protocol stopped")
	}
	c.failOnce.Do(func() {
		c.stateMu.Lock()
		c.readerErr = err
		pending := c.pending
		c.pending = make(map[uint64]chan callResult)
		close(c.done)
		c.stateMu.Unlock()
		for _, waiter := range pending {
			waiter <- callResult{err: err}
		}
		c.closeTransports()
	})
}

func (c *Client) closeTransports() {
	c.transportOnce.Do(func() {
		if closer, ok := c.Writer.(io.Closer); ok {
			_ = closer.Close()
		}
		if closer, ok := c.Reader.(io.Closer); ok {
			_ = closer.Close()
		}
	})
}

func (c *Client) failure() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	if c.readerErr != nil {
		return c.readerErr
	}
	return domain.NewError(domain.CodeProviderFailed, "codex protocol stopped")
}

func (c *Client) waitDuration() time.Duration {
	if c.MaxWait > 0 {
		return c.MaxWait
	}
	return 5 * time.Second
}

func (c *Client) Close() {
	if c != nil {
		c.fail(domain.NewError(domain.CodeProviderFailed, "codex protocol closed"))
	}
}
