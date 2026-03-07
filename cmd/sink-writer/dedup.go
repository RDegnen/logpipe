package main

import (
	"context"
	"log"

	"github.com/Rdegnen/logpipe/internal/config"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Dedup struct {
	SeenEventsMap map[string]bool
}

func (d *Dedup) Bootstrap(ctx context.Context, kafkaClient *kgo.Client) {
	cfg := config.Load()
	admin := kadm.NewClient(kafkaClient)
	defer admin.Close()

	endOffsets, err := admin.ListEndOffsets(ctx, cfg.SeenEventsTopic)
	if err != nil {
		log.Printf("error fetching end offsets: topic=%s", cfg.SeenEventsTopic)
	}

	fetches := kafkaClient.PollFetches(ctx)
	if errs := fetches.Errors(); len(errs) > 0 {
		for _, fe := range errs {
			// Common transient errors; log and keep going
			log.Printf("fetch error: topic=%s partition=%d err=%v", fe.Topic, fe.Partition, fe.Err)
		}
	}

	fetches.EachRecord(func(record *kgo.Record) {

	})
}

func (d *Dedup) IsDuplicate(eventId string) bool {
	return d.SeenEventsMap[eventId]
}

func (d *Dedup) Mark(eventId string) {
	d.SeenEventsMap[eventId] = true
}
