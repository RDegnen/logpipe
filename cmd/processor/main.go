package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Rdegnen/logpipe/internal/config"
	"github.com/Rdegnen/logpipe/internal/utilities"
	"github.com/Rdegnen/logpipe/pkg/logevent"
	"github.com/twmb/franz-go/pkg/kgo"
)

type dlqMessage struct {
	Reason      string `json:"reason"`
	Error       string `json:"error,omitempty"`
	Topic       string `json:"topic"`
	Partition   int32  `json:"partition"`
	Offset      int64  `json:"offset"`
	KeyB64      string `json:"key_b64,omitempty"`
	ValueB64    string `json:"value_b64,omitempty"`
	ProducedAt  string `json:"produced_at"`
	ConsumerGrp string `json:"consumer_group"`
}

func main() {
	cfg := config.Load()
	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.KafkaBrokers),
		kgo.ConsumerGroup(cfg.KafkaProcessorGroup),
		kgo.ConsumeTopics(cfg.RawLogsTopic),
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.DisableAutoCommit(),
		kgo.ProducerLinger(10*time.Millisecond),
		kgo.ProducerBatchMaxBytes(1<<20),
	)
	if err != nil {
		log.Fatalf("kafka client: %v", err)
	}
	defer kafkaClient.Close()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("processor v1 starting: brokers=%v group=%s in=%s out=%s dlq=%s",
		cfg.KafkaBrokers, cfg.KafkaProcessorGroup, cfg.RawLogsTopic, cfg.ProcessedLogsTopic, cfg.DLQTopic,
	)

	var handledSinceCommit int
	for ctx.Err() == nil {
		fetches := kafkaClient.PollFetches(ctx)
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, fe := range errs {
				// Common transient errors; log and keep going
				log.Printf("fetch error: topic=%s partition=%d err=%v", fe.Topic, fe.Partition, fe.Err)
			}
			continue
		}

		// Track which offsets are safe to commit (highest contiguous per partition)
		// We process records in the order franz-go delivers per partition.
		commitMap := make(map[string]map[int32]kgo.EpochOffset)
		var stopProcessing bool

		fetches.EachRecord(func(rec *kgo.Record) {
			if stopProcessing || ctx.Err() != nil {
				return
			}

			handled, err := processOne(ctx, kafkaClient, cfg, rec)
			if err != nil {
				// If this was a transient produce issue, do NOT mark for commit.
				// We'll retry on next poll.
				log.Printf("process error (will retry): topic=%s partition=%d offset=%d err=%v",
					rec.Topic, rec.Partition, rec.Offset, err)
				stopProcessing = true
				return
			}

			if handled {
				handledSinceCommit++
				// Mark this record as safe to commit.
				if commitMap[rec.Topic] == nil {
					commitMap[rec.Topic] = make(map[int32]kgo.EpochOffset)
				}
				commitMap[rec.Topic][rec.Partition] = kgo.EpochOffset{
					Epoch:  rec.LeaderEpoch,
					Offset: rec.Offset + 1, // Kafka commits "next offset"
				}
			}

			// Commit occasionally to avoid huge replays on crash.
			if cfg.CommitEvery > 0 && handledSinceCommit >= cfg.CommitEvery {
				utilities.CommitOffsets(ctx, kafkaClient, commitMap)
				handledSinceCommit = 0
				clear(commitMap)
			}
		})

		// Commit whatever we have from this poll batch.
		if len(commitMap) > 0 {
			utilities.CommitOffsets(ctx, kafkaClient, commitMap)
			handledSinceCommit = 0
		}
	}

	log.Printf("processor v1 stopping")
}

func processOne(ctx context.Context, kafkaClient *kgo.Client, cfg *config.Config, rec *kgo.Record) (handled bool, err error) {
	// 1) Unmarshal envelope
	var evt logevent.LogEvent
	if uerr := json.Unmarshal(rec.Value, &evt); uerr != nil {
		// Poison / malformed envelope => DLQ and commit
		if derr := produceDLQ(ctx, kafkaClient, cfg, rec, "unmarshal_error", uerr); derr != nil {
			// DLQ produce failed => treat as transient (don't commit)
			return false, derr
		}
		return true, nil
	}

	// 2) Minimal validation
	if verr := validateEvent(evt); verr != nil {
		if derr := produceDLQ(ctx, kafkaClient, cfg, rec, "validation_error", verr); derr != nil {
			return false, derr
		}
		return true, nil
	}

	// 3) Enrich (tiny v1 example)
	if evt.Body == nil {
		evt.Body = map[string]any{}
	}
	evt.Body["_processed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	evt.Body["_processor_version"] = "v1"

	// 4) Produce to output topic (sync so "handled" means acked)
	outBytes, merr := json.Marshal(evt)
	if merr != nil {
		// treat as poison; DLQ and commit
		if derr := produceDLQ(ctx, kafkaClient, cfg, rec, "marshal_error", merr); derr != nil {
			return false, derr
		}
		return true, nil
	}

	outRecord := &kgo.Record{
		Topic: cfg.ProcessedLogsTopic,
		Key:   rec.Key,
		Value: outBytes,
	}

	if perr := utilities.ProduceSync(ctx, kafkaClient, outRecord); perr != nil {
		// transient => retry later (do not commit)
		return false, perr
	}

	return true, nil

}

func validateEvent(e logevent.LogEvent) error {
	if e.SchemaVersion != 1 {
		return errors.New("unsupported schema_version: " + strconv.Itoa(e.SchemaVersion))
	}
	if strings.TrimSpace(e.TenantID) == "" {
		return errors.New("missing tenant_id")
	}
	if strings.TrimSpace(e.EventID) == "" {
		return errors.New("missing event_id")
	}
	if e.ObservedAt.IsZero() {
		return errors.New("missing observed_at")
	}
	if strings.TrimSpace(e.Level) == "" {
		return errors.New("missing level")
	}
	if strings.TrimSpace(e.Source.Service) == "" {
		return errors.New("missing source.service")
	}
	return nil
}

func produceDLQ(ctx context.Context, kafkaClient *kgo.Client, cfg *config.Config, rec *kgo.Record, reason string, cause error) error {
	msg := dlqMessage{
		Reason:      reason,
		Error:       cause.Error(),
		Topic:       rec.Topic,
		Partition:   rec.Partition,
		Offset:      rec.Offset,
		KeyB64:      base64.StdEncoding.EncodeToString(rec.Key),
		ValueB64:    base64.StdEncoding.EncodeToString(rec.Value),
		ProducedAt:  time.Now().UTC().Format(time.RFC3339Nano),
		ConsumerGrp: cfg.KafkaProcessorGroup,
	}
	b, _ := json.Marshal(msg)

	dlqRec := &kgo.Record{
		Topic: cfg.DLQTopic,
		// Keep same key so DLQ can be partitioned by tenant too
		Key:   rec.Key,
		Value: b,
	}
	return utilities.ProduceSync(ctx, kafkaClient, dlqRec)
}
