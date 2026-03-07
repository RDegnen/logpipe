package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/Rdegnen/logpipe/internal/config"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	cfg := config.Load()
	bootstrapKafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.KafkaBrokers),
		kgo.ConsumeTopics(cfg.SeenEventsTopic),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		log.Fatalf("bootstrap kafka client: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	dedup := Dedup{SeenEventsMap: make(map[string]bool)}
	dedup.Bootstrap(ctx, bootstrapKafkaClient, cfg.SeenEventsTopic)
	bootstrapKafkaClient.Close()
}
