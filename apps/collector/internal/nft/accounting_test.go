package nft

import "testing"

func TestCounterSnapshotDeltas(t *testing.T) {
	current := CounterSnapshot{InternetDownload: 100, InternetUpload: 50, LANDownload: 20}
	previous := CounterSnapshot{InternetDownload: 40, InternetUpload: 60, LANDownload: 10}

	got := current.Deltas(previous)
	if got.InternetDownload != 60 {
		t.Fatalf("expected internet download delta 60, got %d", got.InternetDownload)
	}
	if got.InternetUpload != 0 {
		t.Fatalf("counter reset or lower value should produce zero delta, got %d", got.InternetUpload)
	}
	if got.LANDownload != 10 {
		t.Fatalf("expected lan download delta 10, got %d", got.LANDownload)
	}
}
