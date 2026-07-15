package runtime

import (
	"sync"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

const (
	DefaultRingBytes = 4 * 1024 * 1024
	DefaultChunkSize = 4 * 1024
)

// Chunk is one bounded, monotonic replay unit. Data is copied at the API
// boundary so callers cannot mutate retained output.
type Chunk struct {
	Sequence  int64
	Data      []byte
	Truncated bool
}

type Replay struct {
	Chunks       []Chunk
	Truncated    bool
	NextSequence int64
}

// RingBuffer retains output by bytes, not by number of writes. Every append
// is split into fixed-size chunks so a provider cannot bypass the quota with a
// single oversized write.
type RingBuffer struct {
	mu        sync.RWMutex
	capacity  int
	chunkSize int
	chunks    []Chunk
	bytes     int
	next      int64
}

func NewRingBuffer(capacity, chunkSize int) (*RingBuffer, error) {
	if capacity <= 0 || chunkSize <= 0 || capacity < chunkSize {
		return nil, domain.NewError(domain.CodeInvalidArgument, "ring buffer bounds are invalid")
	}
	return &RingBuffer{capacity: capacity, chunkSize: chunkSize}, nil
}

func NewDefaultRingBuffer() *RingBuffer {
	ring, _ := NewRingBuffer(DefaultRingBytes, DefaultChunkSize)
	return ring
}

func (r *RingBuffer) Append(data []byte) []Chunk {
	if r == nil || len(data) == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	added := make([]Chunk, 0, (len(data)+r.chunkSize-1)/r.chunkSize)
	for len(data) > 0 {
		n := r.chunkSize
		if len(data) < n {
			n = len(data)
		}
		chunk := Chunk{Sequence: r.next + 1, Data: append([]byte(nil), data[:n]...)}
		r.next = chunk.Sequence
		r.chunks = append(r.chunks, chunk)
		r.bytes += len(chunk.Data)
		added = append(added, cloneChunk(chunk))
		data = data[n:]
	}
	for r.bytes > r.capacity && len(r.chunks) > 0 {
		removed := r.chunks[0]
		r.chunks = r.chunks[1:]
		r.bytes -= len(removed.Data)
	}
	return added
}

func (r *RingBuffer) Replay(from int64) (Replay, error) {
	if r == nil || from < 0 {
		return Replay{}, domain.NewError(domain.CodeInvalidArgument, "replay sequence is invalid")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := Replay{NextSequence: r.next + 1}
	if len(r.chunks) == 0 {
		return result, nil
	}
	oldest := r.chunks[0].Sequence
	if from == 0 {
		from = oldest
	}
	if from < oldest {
		result.Truncated = true
		result.Chunks = cloneChunks(r.chunks)
		return result, domain.NewError(domain.CodeReplayUnavailable, "requested replay is no longer retained")
	}
	for _, chunk := range r.chunks {
		if chunk.Sequence >= from {
			result.Chunks = append(result.Chunks, cloneChunk(chunk))
		}
	}
	return result, nil
}

func (r *RingBuffer) Snapshot() Replay {
	result, _ := r.Replay(0)
	return result
}

func cloneChunk(chunk Chunk) Chunk {
	chunk.Data = append([]byte(nil), chunk.Data...)
	return chunk
}

func cloneChunks(chunks []Chunk) []Chunk {
	result := make([]Chunk, 0, len(chunks))
	for _, chunk := range chunks {
		result = append(result, cloneChunk(chunk))
	}
	return result
}
