package firehose

import (
	"context"
	"testing"
	"time"

	"github.com/bluesky-social/indigo/api/atproto"
)

type testHandler struct {
	events int
}

func (h *testHandler) HandleEvent(evt *atproto.SyncSubscribeRepos_Commit) error {
	h.events++
	return nil
}

func TestFirehose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	f, err := New("wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos")
	if err != nil {
		t.Fatal(err)
	}

	handler := &testHandler{}

	go func() {
		if err := f.Subscribe(ctx, handler); err != nil {
			t.Logf("Subscribe ended: %v", err)
		}
	}()

	// Wait a bit to collect some events
	time.Sleep(3 * time.Second)
	f.Close()

	t.Logf("Received %d events", handler.events)
}
