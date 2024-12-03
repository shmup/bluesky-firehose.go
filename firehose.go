// Package firehose provides a client for consuming the Bluesky firehose API stream
package firehose

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/bluesky-social/indigo/api/atproto"
	"github.com/bluesky-social/indigo/events"
	"github.com/bluesky-social/indigo/events/schedulers/sequential"
	"github.com/gorilla/websocket"
)

type JetstreamPost struct {
	DID    string `json:"did"`
	Time   int64  `json:"time_us"`
	Type   string `json:"type"`
	Kind   string `json:"kind"`
	Commit struct {
		Record struct {
			Text string `json:"text"`
		} `json:"record"`
	} `json:"commit"`
}

// FirehoseHandler defines the interface for handling firehose events
type FirehoseHandler interface {
	HandleEvent(*atproto.SyncSubscribeRepos_Commit) error
}

// Firehose represents a client connection to the Bluesky firehose
type Firehose struct {
	conn        *websocket.Conn
	accessToken string
}

// Post represents the structure of a Bluesky post response
type Post struct {
	Thread struct {
		Post struct {
			Record struct {
				Text string `json:"text"`
			}
		}
	}
}

type postHandler struct {
	firehose *Firehose
	handler  func(string) error
}

// New creates a new Firehose client connected to the specified websocket URL
func New(wsURL string) (*Firehose, error) {
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return nil, err
	}
	return &Firehose{conn: conn}, nil
}

// Subscribe starts consuming the firehose stream with the provided handler
func (f *Firehose) Subscribe(ctx context.Context, handler FirehoseHandler) error {
	callbacks := &events.RepoStreamCallbacks{
		RepoCommit: handler.HandleEvent,
	}
	return events.HandleRepoStream(ctx, f.conn, sequential.NewScheduler("firehose", callbacks.EventHandler))
}

// OnPost registers a callback function to handle new posts from the firehose
func (f *Firehose) OnPost(ctx context.Context, handler func(string) error) error {
	return f.Subscribe(ctx, &postHandler{
		firehose: f,
		handler:  handler,
	})
}

// Authenticate logs into Bluesky using email/password and stores the access token
func (f *Firehose) Authenticate(email, password string) error {
	if email == "" || password == "" {
		return fmt.Errorf("email and password required")
	}

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

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}

	var auth struct{ AccessJwt string }
	if err := json.NewDecoder(resp.Body).Decode(&auth); err != nil {
		return err
	}

	if auth.AccessJwt == "" {
		return fmt.Errorf("no access token received")
	}

	f.accessToken = auth.AccessJwt
	return nil
}

// HandleEvent processes individual commit events from the firehose stream
func (h *postHandler) HandleEvent(evt *atproto.SyncSubscribeRepos_Commit) error {
	for _, op := range evt.Ops {
		if op.Action == "create" && strings.HasPrefix(op.Path, "app.bsky.feed.post") {
			uri := fmt.Sprintf("at://%s/app.bsky.feed.post/%s",
				evt.Repo, strings.TrimPrefix(op.Path, "app.bsky.feed.post/"))
			if text, err := h.firehose.FetchPost(uri); err == nil {
				if err := h.handler(text); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// FetchPost retrieves the text content of a post by its URI
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

// Close terminates the firehose connection
func (f *Firehose) Close() error {
	return f.conn.Close()
}

// ConsumeJetstream consumes the Bluesky Jetstream API stream with the provided handler
func (f *Firehose) ConsumeJetstream(ctx context.Context, handler func(JetstreamPost) error, errCallback ...func(error)) error {
	conn, _, err := websocket.DefaultDialer.Dial(
		"wss://jetstream2.us-east.bsky.network/subscribe?wantedCollections=app.bsky.feed.post",
		nil,
	)
	if err != nil {
		if len(errCallback) > 0 {
			errCallback[0](err)
		}
		return err
	}
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			if len(errCallback) > 0 {
				errCallback[0](ctx.Err())
			}
			return ctx.Err()
		default:
			_, message, err := conn.ReadMessage()
			if err != nil {
				if len(errCallback) > 0 {
					errCallback[0](err)
				}
				return err
			}

			var post JetstreamPost
			if err := json.Unmarshal(message, &post); err != nil {
				if len(errCallback) > 0 {
					errCallback[0](err)
				}
				continue
			}

			if err := handler(post); err != nil {
				if len(errCallback) > 0 {
					errCallback[0](err)
				}
				return err
			}
		}
	}
}
