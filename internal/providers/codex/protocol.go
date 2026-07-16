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
	Reader       io.Reader
	Writer       io.Writer
	MaxWait      time.Duration
	mu           sync.Mutex
	nextID       uint64
	handshaken   bool
	capabilities map[string]struct{}
	frames       *FrameReader
	inbound      []json.RawMessage
}

func NewClient(reader io.Reader, writer io.Writer) *Client {
	return &Client{Reader: reader, Writer: writer, MaxWait: 5 * time.Second, nextID: 1,
		capabilities: make(map[string]struct{}), frames: NewFrameReader(reader)}
}

// ConfigureMethods installs the exact schema-derived allowlist before the
// initialize handshake. The current app-server initialize response does not
// negotiate or return a method list.
func (c *Client) ConfigureMethods(methods []string) error {
	if c == nil {
		return domain.NewError(domain.CodeInvalidArgument, "codex protocol client is unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handshaken {
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
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handshaken {
		return InitializeResult{}, domain.NewError(domain.CodeConflict, "codex protocol handshake already completed")
	}
	if strings.TrimSpace(params.ClientInfo.Name) == "" || strings.TrimSpace(params.ClientInfo.Version) == "" || params.Capabilities == nil || params.Capabilities.ExperimentalAPI {
		return InitializeResult{}, domain.NewError(domain.CodeInvalidArgument, "codex initialize parameters are invalid")
	}
	id := c.nextID
	c.nextID++
	if err := WriteFrame(c.Writer, RPCRequest{JSONRPC: "2.0", ID: id, Method: MethodInitialize, Params: params}); err != nil {
		return InitializeResult{}, err
	}
	frame, err := c.readResponseLocked(ctx, id)
	if err != nil {
		return InitializeResult{}, err
	}
	var response RPCResponse
	if err := DecodeObject(frame, &response); err != nil {
		return InitializeResult{}, err
	}
	if response.Error != nil {
		return InitializeResult{}, domain.NewError(domain.CodeProviderProtocolError, "codex initialize was rejected")
	}
	if string(response.ID) != strconv.FormatUint(id, 10) {
		return InitializeResult{}, domain.NewError(domain.CodeProviderProtocolError, "codex initialize response id did not match")
	}
	var result InitializeResult
	if len(response.Result) > 0 {
		if err := DecodeObject(response.Result, &result); err != nil {
			return InitializeResult{}, err
		}
	}
	if strings.TrimSpace(result.UserAgent) == "" || strings.TrimSpace(result.CodexHome) == "" ||
		strings.TrimSpace(result.PlatformFamily) == "" || strings.TrimSpace(result.PlatformOS) == "" ||
		len(result.UserAgent) > 512 || len(result.CodexHome) > 4096 || len(result.PlatformFamily) > 64 || len(result.PlatformOS) > 64 {
		return InitializeResult{}, domain.NewError(domain.CodeProviderProtocolError, "codex initialize result is invalid")
	}
	if err := WriteFrame(c.Writer, RPCRequest{JSONRPC: "2.0", Method: MethodInitialized}); err != nil {
		return InitializeResult{}, err
	}
	c.handshaken = true
	return result, nil
}

// RespondServerRequest writes a JSON-RPC result for a Provider-initiated
// request. The raw request ID is preserved but never interpreted as authority.
func (c *Client) RespondServerRequest(ctx context.Context, id json.RawMessage, result any) error {
	if c == nil || c.Writer == nil || ctx == nil || len(id) == 0 || len(id) > 256 {
		return domain.NewError(domain.CodeInvalidArgument, "codex server response is incomplete")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.handshaken {
		return domain.NewError(domain.CodeProviderProtocolError, "codex initialize handshake is required")
	}
	select {
	case <-ctx.Done():
		return domain.NewError(domain.CodeDeadlineExceeded, "codex server response was cancelled")
	default:
	}
	return WriteFrame(c.Writer, RPCResponse{JSONRPC: "2.0", ID: id, Result: mustRawJSON(result)})
}

func mustRawJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func (c *Client) Call(ctx context.Context, method string, params any, result any) error {
	if c == nil || c.Reader == nil || c.Writer == nil || ctx == nil || method == "" {
		return domain.NewError(domain.CodeInvalidArgument, "codex protocol call is incomplete")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.handshaken {
		return domain.NewError(domain.CodeProviderProtocolError, "codex initialize handshake is required")
	}
	if method != MethodInitialize && method != MethodInitialized {
		if _, ok := c.capabilities[method]; !ok {
			return domain.NewError(domain.CodeProviderVersionUnsupported, "codex method is not enabled")
		}
	}
	id := c.nextID
	c.nextID++
	if err := WriteFrame(c.Writer, RPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}); err != nil {
		return err
	}
	frame, err := c.readResponseLocked(ctx, id)
	if err != nil {
		return err
	}
	var response RPCResponse
	if err := DecodeObject(frame, &response); err != nil {
		return err
	}
	if response.Error != nil {
		return domain.NewError(domain.CodeProviderProtocolError, "codex request was rejected")
	}
	if string(response.ID) != strconv.FormatUint(id, 10) {
		return domain.NewError(domain.CodeProviderProtocolError, "codex response id did not match")
	}
	if result != nil && len(response.Result) > 0 {
		if err := DecodeObject(response.Result, result); err != nil {
			return err
		}
	}
	return nil
}

// ReadInbound returns Provider-initiated notifications or server requests in
// arrival order. Responses observed while waiting on a client call are never
// mistaken for events.
func (c *Client) ReadInbound(ctx context.Context) (json.RawMessage, error) {
	if c == nil || ctx == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "codex inbound read is incomplete")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.inbound) > 0 {
		frame := c.inbound[0]
		c.inbound = c.inbound[1:]
		return frame, nil
	}
	return c.read(ctx)
}

func (c *Client) readResponseLocked(ctx context.Context, expectedID uint64) (json.RawMessage, error) {
	for {
		frame, err := c.read(ctx)
		if err != nil {
			return nil, err
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
			return nil, err
		}
		if envelope.Method != "" {
			if len(c.inbound) >= 64 {
				return nil, domain.NewError(domain.CodeResourceExhausted, "codex inbound event queue is full")
			}
			c.inbound = append(c.inbound, append(json.RawMessage(nil), frame...))
			continue
		}
		if string(envelope.ID) != strconv.FormatUint(expectedID, 10) {
			return nil, domain.NewError(domain.CodeProviderProtocolError, "codex response id did not match")
		}
		return frame, nil
	}
}

func (c *Client) read(ctx context.Context) ([]byte, error) {
	if c.frames == nil {
		c.frames = NewFrameReader(c.Reader)
	}
	return readWithContext(ctx, c.frames, c.MaxWait)
}

type frameSource interface {
	Read() (json.RawMessage, error)
}

func readWithContext(ctx context.Context, reader frameSource, timeout time.Duration) ([]byte, error) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	result := make(chan struct {
		frame json.RawMessage
		err   error
	}, 1)
	go func() {
		frame, err := reader.Read()
		result <- struct {
			frame json.RawMessage
			err   error
		}{frame: frame, err: err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, domain.NewError(domain.CodeDeadlineExceeded, "codex protocol call was cancelled")
	case <-timer.C:
		return nil, domain.NewError(domain.CodeDeadlineExceeded, "codex protocol call timed out")
	case response := <-result:
		if response.err != nil {
			return nil, response.err
		}
		return response.frame, nil
	}
}
