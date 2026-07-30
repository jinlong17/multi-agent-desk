package transport

import (
	"encoding/json"
	"fmt"
	"time"
)

// BootstrapKeyEnvelopeAssertionJCSV1 returns the restricted RFC 8785 payload
// for the five scalar fields frozen by the P2 bootstrap protocol. The keys are
// declared in lexicographic order and the accepted values cannot contain the
// JSON string cases for which encoding/json and JCS escaping differ.
func BootstrapKeyEnvelopeAssertionJCSV1(formatVersion, keyRevision, recordRevision int, sealedAt time.Time, status string) ([]byte, error) {
	if formatVersion != 1 || keyRevision != 1 || recordRevision < 1 || sealedAt.IsZero() || sealedAt.Location() != time.UTC || sealedAt.Nanosecond()%1_000 != 0 || status != "pending" {
		return nil, fmt.Errorf("bootstrap key-envelope assertion is invalid")
	}
	value := struct {
		FormatVersion  int       `json:"formatVersion"`
		KeyRevision    int       `json:"keyRevision"`
		RecordRevision int       `json:"recordRevision"`
		SealedAt       time.Time `json:"sealedAt"`
		Status         string    `json:"status"`
	}{formatVersion, keyRevision, recordRevision, sealedAt, status}
	return json.Marshal(value)
}
