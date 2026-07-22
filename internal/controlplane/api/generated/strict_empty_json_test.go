package generated

import (
	"encoding/json"
	"testing"

	"github.com/jinlong17/multi-agent-desk/internal/transport"
)

func TestP2EmptyObjectRequestV1IsStrictTransportAlias(t *testing.T) {
	var transportValue transport.EmptyJSONObjectV1
	var generatedValue P2EmptyObjectRequestV1 = transportValue
	transportValue = generatedValue

	for _, valid := range []string{"{}", " \n { \t } \r\n"} {
		if err := json.Unmarshal([]byte(valid), &generatedValue); err != nil {
			t.Fatalf("valid body %q rejected: %v", valid, err)
		}
		encoded, err := json.Marshal(generatedValue)
		if err != nil {
			t.Fatalf("marshal strict empty body: %v", err)
		}
		if string(encoded) != "{}" {
			t.Fatalf("marshal=%s want={}", encoded)
		}
	}

	for _, hostile := range []string{
		`{"extra":true}`,
		`null`,
		`[]`,
		`{} {}`,
		``,
	} {
		if err := json.Unmarshal([]byte(hostile), &generatedValue); err == nil {
			t.Fatalf("hostile body %q accepted", hostile)
		}
	}
}
