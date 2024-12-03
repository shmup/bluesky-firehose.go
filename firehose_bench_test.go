package firehose

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
)

type benchHandler struct {
	count int
}

func (h *benchHandler) HandleEvent(evt *atproto.SyncSubscribeRepos_Commit) error {
	h.count++
	return nil
}

func BenchmarkFirehose(b *testing.B) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	f, err := New("wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos")
	if err != nil {
		b.Fatal(err)
	}

	handler := &benchHandler{}

	b.ResetTimer()
	go func() {
		if err := f.Subscribe(ctx, handler); err != nil {
			b.Logf("Subscribe ended: %v", err)
		}
	}()

	time.Sleep(time.Second)
	f.Close()
	b.StopTimer()

	b.ReportMetric(float64(handler.count), "events/op")
}
