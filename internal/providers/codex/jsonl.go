package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func ReadFrame(reader io.Reader) (json.RawMessage, error) {
	if reader == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "jsonl reader is required")
	}
	return NewFrameReader(reader).Read()
}

type FrameReader struct {
	reader *bufio.Reader
}

func NewFrameReader(reader io.Reader) *FrameReader {
	return &FrameReader{reader: bufio.NewReaderSize(reader, 32*1024)}
}

func (f *FrameReader) Read() (json.RawMessage, error) {
	if f == nil || f.reader == nil {
		return nil, domain.NewError(domain.CodeInvalidArgument, "jsonl reader is required")
	}
	line, err := f.reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, domain.WrapError(domain.CodeProviderProtocolError, "jsonl frame could not be read", err)
	}
	if len(line) == 0 && errors.Is(err, io.EOF) {
		return nil, domain.NewError(domain.CodeProviderProtocolError, "jsonl stream ended before a frame")
	}
	frame := bytes.TrimSpace([]byte(strings.TrimSuffix(line, "\n")))
	if len(frame) == 0 || len(frame) > MaxFrameBytes || !utf8.Valid(frame) {
		return nil, domain.NewError(domain.CodeProviderProtocolError, "jsonl frame is invalid")
	}
	if err := validateJSON(frame); err != nil {
		return nil, err
	}
	return append(json.RawMessage(nil), frame...), nil
}

func WriteFrame(writer io.Writer, value any) error {
	if writer == nil {
		return domain.NewError(domain.CodeInvalidArgument, "jsonl writer is required")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return domain.NewError(domain.CodeInvalidArgument, "jsonl value could not be encoded")
	}
	if len(encoded) == 0 || len(encoded) > MaxFrameBytes || !utf8.Valid(encoded) {
		return domain.NewError(domain.CodeFrameTooLarge, "jsonl frame is too large")
	}
	if err := writeAll(writer, append(encoded, '\n')); err != nil {
		return domain.WrapError(domain.CodeProviderProtocolError, "jsonl frame could not be written", err)
	}
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := writer.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(data) {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

func DecodeObject(frame []byte, target any) error {
	if target == nil {
		return domain.NewError(domain.CodeInvalidArgument, "jsonl decode target is required")
	}
	if err := validateJSON(frame); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(frame))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return domain.NewError(domain.CodeProviderProtocolError, "jsonl object does not match the negotiated schema")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return domain.NewError(domain.CodeProviderProtocolError, "jsonl frame contains trailing values")
	}
	return nil
}

func validateJSON(frame []byte) error {
	if len(frame) == 0 || len(frame) > MaxFrameBytes || !utf8.Valid(frame) {
		return domain.NewError(domain.CodeFrameTooLarge, "jsonl frame is too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(frame))
	if err := validateValue(decoder); err != nil {
		return domain.NewError(domain.CodeProviderProtocolError, "jsonl frame contains invalid or duplicate fields")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return domain.NewError(domain.CodeProviderProtocolError, "jsonl frame contains trailing values")
	}
	return nil
}

func validateValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	switch delimiter := token.(type) {
	case json.Delim:
		switch delimiter {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return err
				}
				name, ok := key.(string)
				if !ok {
					return errors.New("object key is not a string")
				}
				if _, exists := seen[name]; exists {
					return errors.New("duplicate object key")
				}
				seen[name] = struct{}{}
				if err := validateValue(decoder); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return errors.New("object is not closed")
			}
		case '[':
			for decoder.More() {
				if err := validateValue(decoder); err != nil {
					return err
				}
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return errors.New("array is not closed")
			}
		default:
			return errors.New("unexpected JSON delimiter")
		}
	}
	return nil
}
