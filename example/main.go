package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"

	bsky "github.com/shmup/bluesky-firehose.go"
)

func main() {
	firehose, err := bsky.New("wss://bsky.network/xrpc/com.atproto.sync.subscribeRepos")
	if err != nil {
		log.Fatal(err)
	}
	godotenv.Load()

	if err := firehose.Authenticate(os.Getenv("BSKY_EMAIL"), os.Getenv("BSKY_PASSWORD")); err != nil {
		log.Fatal(err)
	}

	firehose.OnPost(context.Background(), func(text string) error {
		fmt.Printf("%s\n", text)

		return nil
	})
}
