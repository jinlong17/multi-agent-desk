package device

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

type Listener interface {
	Accept() (io.ReadWriteCloser, error)
	Close() error
	Address() string
}

func endpointName(root string) string {
	digest := sha256.Sum256([]byte(root))
	return "multidesk-" + hex.EncodeToString(digest[:10])
}
