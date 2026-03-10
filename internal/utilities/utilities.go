package utilities

import (
	"context"
	"log"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

func ProduceSync(ctx context.Context, kafkaClient *kgo.Client, rec *kgo.Record) error {
	done := make(chan error, 1)
	kafkaClient.Produce(ctx, rec, func(_ *kgo.Record, err error) {
		done <- err
	})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func CommitOffsets(ctx context.Context, kafkaClient *kgo.Client, offsets map[string]map[int32]kgo.EpochOffset) {
	kafkaClient.CommitOffsets(ctx, offsets, func(_ *kgo.Client, _ *kmsg.OffsetCommitRequest, _ *kmsg.OffsetCommitResponse, err error) {
		if err != nil {
			log.Printf("commit error: %v", err)
		}
	})
}
