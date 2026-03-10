package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Rdegnen/logpipe/internal/config"
	"github.com/Rdegnen/logpipe/internal/utilities"
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

	sinkWriterKafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.KafkaBrokers),
		kgo.ConsumeTopics(cfg.ProcessedLogsTopic),
		kgo.ConsumerGroup(cfg.KafkaSinkGroup),
		kgo.DisableAutoCommit(),
		kgo.ProducerLinger(10*time.Millisecond),
		kgo.ProducerBatchMaxBytes(1<<20),
	)
	if err != nil {
		log.Fatalf("sink writer kafka client: %v", err)
	}
	defer sinkWriterKafkaClient.Close()

	var handledSinceCommit int
	for ctx.Err() == nil {
		fetches := sinkWriterKafkaClient.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				// Common transient errors; log and keep going
				log.Printf("fetch error: topic=%s partition=%d err=%v", fe.Topic, fe.Partition, fe.Err)
			}
		}

		commitMap := make(map[string]map[int32]kgo.EpochOffset)
		var stopProcessing bool

		fetches.EachRecord(func(rec *kgo.Record) {
			if stopProcessing || ctx.Err() != nil {
				return
			}

			if dedup.IsDuplicate(string(rec.Key)) {
				return
			}

			// Placeholder for db write
			log.Printf("record: %s", rec.Key)

			perr := produceSeenEvent(ctx, sinkWriterKafkaClient, cfg, rec)
			if perr != nil {
				log.Printf("process error (will retry): topic=%s partition=%d offset=%d err=%v",
					rec.Topic, rec.Partition, rec.Offset, perr)
				stopProcessing = true
				return
			}

			dedup.Mark(string(rec.Key))
			handledSinceCommit++
			// Mark this record as safe to commit.
			if commitMap[rec.Topic] == nil {
				commitMap[rec.Topic] = make(map[int32]kgo.EpochOffset)
			}
			commitMap[rec.Topic][rec.Partition] = kgo.EpochOffset{
				Epoch:  rec.LeaderEpoch,
				Offset: rec.Offset + 1, // Kafka commits "next offset"
			}

			if cfg.CommitEvery > 0 && handledSinceCommit >= cfg.CommitEvery {
				utilities.CommitOffsets(ctx, sinkWriterKafkaClient, commitMap)
				handledSinceCommit = 0
				clear(commitMap)
			}
		})

		if len(commitMap) > 0 {
			utilities.CommitOffsets(ctx, sinkWriterKafkaClient, commitMap)
			handledSinceCommit = 0
		}
	}
}

func produceSeenEvent(ctx context.Context, kafkaClient *kgo.Client, cfg *config.Config, rec *kgo.Record) error {
	seenRec := &kgo.Record{
		Topic: cfg.SeenEventsTopic,
		Key:   rec.Key,
	}
	return utilities.ProduceSync(ctx, kafkaClient, seenRec)
}
