package runtime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const fakeProviderProtocolVersion = 1

type fakeRequest struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Payload string `json:"payload,omitempty"`
	Rows    int    `json:"rows,omitempty"`
	Cols    int    `json:"cols,omitempty"`
}

// ChildEvent is the bounded internal event emitted by the Fake Provider.
// This protocol is deliberately not exposed as a Provider compatibility API.
type ChildEvent struct {
	Version int    `json:"version"`
	Kind    string `json:"kind"`
	Payload string `json:"payload,omitempty"`
	Code    int    `json:"code,omitempty"`
}

// RunFakeProvider is the hidden child entrypoint used by the same multidesk
// binary. It is line framed and deterministic to make process and failure
// behavior testable on all three platforms.
func RunFakeProvider(in io.Reader, out io.Writer) error {
	writer := bufio.NewWriter(out)
	emit := func(event ChildEvent) error {
		encoded, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := writer.Write(append(encoded, '\n')); err != nil {
			return err
		}
		return writer.Flush()
	}
	if err := emit(ChildEvent{Version: fakeProviderProtocolVersion, Kind: "ready"}); err != nil {
		return err
	}
	if err := emit(ChildEvent{Version: fakeProviderProtocolVersion, Kind: "output", Payload: "fake-provider ready\n"}); err != nil {
		return err
	}
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 1024), 64*1024)
	for scanner.Scan() {
		var request fakeRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil || request.Version != fakeProviderProtocolVersion {
			return fmt.Errorf("invalid fake provider request")
		}
		switch request.Kind {
		case "input":
			payload := strings.ReplaceAll(request.Payload, "\n", "")
			if err := emit(ChildEvent{Version: fakeProviderProtocolVersion, Kind: "output", Payload: "echo:" + payload + "\n"}); err != nil {
				return err
			}
		case "resize":
			if request.Rows <= 0 || request.Cols <= 0 {
				return fmt.Errorf("invalid resize")
			}
			if err := emit(ChildEvent{Version: fakeProviderProtocolVersion, Kind: "resized", Payload: strconv.Itoa(request.Rows) + "x" + strconv.Itoa(request.Cols)}); err != nil {
				return err
			}
		case "stop":
			if err := emit(ChildEvent{Version: fakeProviderProtocolVersion, Kind: "exit", Code: 0}); err != nil {
				return err
			}
			return nil
		default:
			return fmt.Errorf("unknown fake provider request")
		}
	}
	return scanner.Err()
}
