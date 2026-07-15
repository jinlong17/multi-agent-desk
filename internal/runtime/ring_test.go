package runtime

import (
	"bytes"
	"testing"

	"github.com/jinlong17/multi-agent-desk/internal/domain"
)

func TestRingBufferChunksAndTruncatesByBytes(t *testing.T) {
	ring, err := NewRingBuffer(8, 4)
	if err != nil {
		t.Fatal(err)
	}
	got := ring.Append([]byte("abcdefghijk"))
	if len(got) != 3 || got[0].Sequence != 1 || got[2].Sequence != 3 {
		t.Fatalf("chunks = %+v", got)
	}
	replay, err := ring.Replay(1)
	if domain.CodeOf(err) != domain.CodeReplayUnavailable || !replay.Truncated {
		t.Fatalf("replay error=%v result=%+v", err, replay)
	}
	if len(replay.Chunks) != 2 || !bytes.Equal(replay.Chunks[0].Data, []byte("efgh")) {
		t.Fatalf("retained chunks = %+v", replay.Chunks)
	}
	if _, err := ring.Replay(-1); domain.CodeOf(err) != domain.CodeInvalidArgument {
		t.Fatalf("negative replay code = %v", domain.CodeOf(err))
	}
}

func TestFakeProviderProtocolIsDeterministic(t *testing.T) {
	var output bytes.Buffer
	input := bytes.NewBufferString(`{"version":1,"kind":"input","payload":"hello"}` + "\n" + `{"version":1,"kind":"resize","rows":24,"cols":80}` + "\n" + `{"version":1,"kind":"stop"}` + "\n")
	if err := RunFakeProvider(input, &output); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`"kind":"ready"`)) ||
		!bytes.Contains(output.Bytes(), []byte(`echo:hello`)) ||
		!bytes.Contains(output.Bytes(), []byte(`24x80`)) ||
		!bytes.Contains(output.Bytes(), []byte(`"kind":"exit"`)) {
		t.Fatalf("unexpected provider output: %s", output.Bytes())
	}
}
