package pihole

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	name string
	args []string
	out  []byte
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	f.name = name
	f.args = args
	return f.out, nil
}

func TestReaderUsesReadonlyIncrementalQuery(t *testing.T) {
	runner := &fakeRunner{out: []byte("1786792800|192.168.50.21|rr1---sn.googlevideo.com|142.250.190.14\n")}
	reader := Reader{DatabasePath: "/readonly/pihole-FTL.db", Runner: runner}

	rows, err := reader.ReadSince(context.Background(), time.Unix(1786792700, 0), 50)
	if err != nil {
		t.Fatal(err)
	}
	if runner.name != "pihole-FTL" || !containsArg(runner.args, "-readonly") {
		t.Fatalf("reader must use pihole-FTL sqlite3 readonly, got %s %+v", runner.name, runner.args)
	}
	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "timestamp > 1786792700") || !strings.Contains(joined, "LIMIT 50") {
		t.Fatalf("query should be incremental and limited, got %s", joined)
	}
	if len(rows) != 1 || rows[0].Domain != "rr1---sn.googlevideo.com" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestParseFTLRowsSkipsMalformedRows(t *testing.T) {
	rows := ParseFTLRows([]byte("bad\n1786792800|192.168.50.21|example.com|203.0.113.10\n"))
	if len(rows) != 1 {
		t.Fatalf("expected one valid row, got %+v", rows)
	}
}

func containsArg(args []string, needle string) bool {
	for _, arg := range args {
		if arg == needle {
			return true
		}
	}
	return false
}
