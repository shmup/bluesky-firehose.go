package firehose

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/sequential"
	"github.com/gorilla/websocket"
)

type FirehoseHandler interface {
	HandleEvent(*atproto.SyncSubscribeRepos_Commit) error
}

type Firehose struct {
	conn        *websocket.Conn
	accessToken string
}

func New(wsURL string) (*Firehose, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	return &Firehose{conn: conn}, nil
}

func (f *Firehose) Subscribe(ctx context.Context, handler FirehoseHandler) error {
	callbacks := &events.RepoStreamCallbacks{
		RepoCommit: handler.HandleEvent,
	}
	return events.HandleRepoStream(ctx, f.conn, sequential.NewScheduler("firehose", callbacks.EventHandler))
}

func (f *Firehose) Close() error {
	return f.conn.Close()
}

// Add to existing firehose.go file or create auth.go:

type Post struct {
	Thread struct {
		Post struct {
			Record struct {
				Text string `json:"text"`
			}
		}
	}
}

func (f *Firehose) Authenticate(email, password string) error {
	body, _ := json.Marshal(map[string]string{
		"identifier": email,
		"password":   password,
	})

	resp, err := http.Post("https://bsky.social/xrpc/com.atproto.server.createSession",
		"application/json", strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var auth struct{ AccessJwt string }
	if err := json.NewDecoder(resp.Body).Decode(&auth); err != nil {
		return err
	}

	f.accessToken = auth.AccessJwt
	return nil
}

func (f *Firehose) FetchPost(uri string) (string, error) {
	req, _ := http.NewRequest("GET", "https://bsky.social/xrpc/app.bsky.feed.getPostThread", nil)
	req.URL.RawQuery = "uri=" + uri
	req.Header.Set("Authorization", "Bearer "+f.accessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var post Post
	if err := json.NewDecoder(resp.Body).Decode(&post); err != nil {
		return "", err
	}
	return post.Thread.Post.Record.Text, nil
}
