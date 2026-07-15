package device

import (
	"context"
	"encoding/json"
	"io"
	"time"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

type Client struct {
	Connection io.ReadWriteCloser
	Auth       AuthOK
}

func Connect(ctx context.Context, root string, identity ClientIdentity, timeout time.Duration) (*Client, error) {
	connection, err := Dial(root, timeout)
	if err != nil {
		return nil, err
	}
	auth, err := (ClientAuthenticator{Identity: identity}).Handshake(ctx, connection)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}
	return &Client{Connection: connection, Auth: auth}, nil
}

func (c *Client) Close() error {
	if c == nil || c.Connection == nil {
		return nil
	}
	return c.Connection.Close()
}

func (c *Client) Call(ctx context.Context, request Request) (Response, error) {
	if c == nil || c.Connection == nil {
		return Response{}, domain.NewError(domain.CodeDaemonUnavailable, "daemon connection is unavailable")
	}
	if err := validateRequest(request, map[string]struct{}{}); err != nil {
		return Response{}, err
	}
	if err := setContextDeadline(c.Connection, ctx, 5*time.Minute); err != nil {
		return Response{}, err
	}
	defer clearDeadline(c.Connection)
	if err := writeFrame(c.Connection, request); err != nil {
		return Response{}, err
	}
	body, err := readFrame(c.Connection)
	if err != nil {
		return Response{}, err
	}
	var response Response
	if err := decodeStrict(body, &response); err != nil || response.ProtocolMajor != ProtocolMajor || response.RequestID != request.RequestID {
		return Response{}, domain.NewError(domain.CodeConflict, "daemon response is invalid")
	}
	if !response.OK && response.Error != nil {
		return response, domain.NewError(response.Error.Code, response.Error.Message)
	}
	return response, nil
}

func JSONBody(value any) (json.RawMessage, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, domain.WrapError(domain.CodeInvalidArgument, "request body is invalid", err)
	}
	if len(encoded) > MaxFrameBytes {
		return nil, domain.NewError(domain.CodeFrameTooLarge, "request body exceeds frame limit")
	}
	return encoded, nil
}
