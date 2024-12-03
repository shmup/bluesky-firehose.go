package main

import (
	"context"
	"fmt"

	firehose "github.com/shmup/bluesky-firehose.go"
)

var client *firehose.Firehose

func main() {
	client, _ = firehose.New("wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos")

	client.ConsumeJetstream(context.Background(), func(post firehose.JetstreamPost) error {
		fmt.Printf("Post from %s: %s\n", post.DID, post.Commit.Record.Text)
		return nil
	})
}
