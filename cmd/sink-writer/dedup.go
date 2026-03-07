package main

import (
	"context"
	"log"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

type Dedup struct {
	SeenEventsMap map[string]bool
}

func (d *Dedup) Bootstrap(ctx context.Context, kafkaClient *kgo.Client, seenEventsTopic string) {
	admin := kadm.NewClient(kafkaClient)
	defer admin.Close()

	endOffsets, err := admin.ListEndOffsets(ctx, seenEventsTopic)
	if err != nil {
		log.Fatalf("error fetching end offsets: topic=%s", seenEventsTopic)
	}

	trackingMap := make(map[int32]int64)
	for _, partitions := range endOffsets {
		for partition, offset := range partitions {
			if offset.Offset > 0 {
				trackingMap[partition] = offset.Offset
			}
		}
	}

	for len(trackingMap) > 0 {
		if ctx.Err() != nil {
			log.Fatalf("bootstrap cancelled: %v", ctx.Err())
		}

		fetches := kafkaClient.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				// Common transient errors; log and keep going
				log.Printf("fetch error: topic=%s partition=%d err=%v", fe.Topic, fe.Partition, fe.Err)
			}
		}

		fetches.EachRecord(func(record *kgo.Record) {
			highWaterMark := trackingMap[record.Partition]

			d.Mark(string(record.Key))
			if record.Offset >= highWaterMark-1 {
				delete(trackingMap, record.Partition)
			}
		})
	}
}

func (d *Dedup) IsDuplicate(eventId string) bool {
	return d.SeenEventsMap[eventId]
}

func (d *Dedup) Mark(eventId string) {
	d.SeenEventsMap[eventId] = true
}
