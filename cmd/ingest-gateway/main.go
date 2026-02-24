package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/oklog/ulid/v2"
	"github.com/twmb/franz-go/pkg/kgo"

	"github.com/Rdegnen/logpipe/pkg/logevent"
)

const (
	topicRawLogs = "raw_logs.v1"
)

type server struct {
	kafka *kgo.Client
}

type ingestResponse struct {
	RequestID string `json:"request_id"`
	Accepted  int    `json:"accepted"`
	Rejected  int    `json:"rejected"`
}

func main() {
	// Kafka client
	k, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9093"),
		// Default is acks=all in Kafka terms? franz-go defaults to -1? We set explicitly:
		kgo.RequiredAcks(kgo.AllISRAcks()),
		// Reasonable batching defaults; tune later:
		kgo.ProducerLinger(10*time.Millisecond),
		kgo.ProducerBatchMaxBytes(1<<20), // 1MB
	)
	if err != nil {
		log.Fatalf("kafka client: %v", err)
	}
	defer k.Close()

	s := &server{kafka: k}

	r := chi.NewRouter()
	r.Post("/ingest", s.handleIngest)

	addr := ":8082"
	log.Printf("ingest-gateway listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func (s *server) handleIngest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-Id"))
	sourceService := strings.TrimSpace(r.Header.Get("X-Source-Service"))
	sourceEnv := strings.TrimSpace(r.Header.Get("X-Source-Env"))
	sourceInstance := strings.TrimSpace(r.Header.Get("X-Source-Instance"))

	if tenantID == "" || sourceService == "" {
		http.Error(w, "missing required headers: X-Tenant-Id, X-Source-Service", http.StatusBadRequest)
		return
	}

	// Content-Type check (be tolerant of charset).
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct != "application/x-ndjson" {
		http.Error(w, "Content-Type must be application/x-ndjson", http.StatusUnsupportedMediaType)
		return
	}

	body, err := maybeGunzip(r.Body, r.Header.Get("Content-Encoding"))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer body.Close()

	reqID := newULID()

	accepted := 0
	rejected := 0

	sc := bufio.NewScanner(body)
	// Increase scan buffer for larger lines.
	sc.Buffer(make([]byte, 64*1024), 2*1024*1024) // 2MB max line

	now := time.Now().UTC()

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			rejected++
			continue
		}

		level, _ := payload["level"].(string)
		if level == "" {
			// Minimal validation: require level; you can tighten later.
			rejected++
			continue
		}

		evt := logevent.LogEvent{
			EventID:  newULID(),
			TenantID: tenantID,
			Source: logevent.Source{
				Service:  sourceService,
				Instance: sourceInstance,
				Env:      sourceEnv,
			},
			ObservedAt:    now,
			SchemaVersion: 1,
			Level:         level,
			Body:          payload,
			Trace: logevent.Trace{
				RequestID: reqID,
			},
		}

		// Optional: pull trace_id / span_id if present in payload
		if v, ok := payload["trace_id"].(string); ok {
			evt.Trace.TraceID = v
		}
		if v, ok := payload["span_id"].(string); ok {
			evt.Trace.SpanID = v
		}

		b, err := json.Marshal(evt)
		if err != nil {
			rejected++
			continue
		}

		rec := &kgo.Record{
			Topic: topicRawLogs,
			Key:   []byte(tenantID),
			Value: b,
		}

		// Synchronous produce so "accepted" means acked.
		// Later we can pipeline and still preserve semantics with careful accounting.
		if err := s.produceSync(ctx, rec); err != nil {
			// For v1, treat produce failures as rejected (or you could 503 the whole request).
			rejected++
			continue
		}
		accepted++
	}

	if err := sc.Err(); err != nil {
		// If the scanner died mid-stream, still return counts for processed lines.
		// You could also return 400.
		log.Printf("scan error: %v", err)
	}

	resp := ingestResponse{
		RequestID: reqID,
		Accepted:  accepted,
		Rejected:  rejected,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *server) produceSync(ctx context.Context, rec *kgo.Record) error {
	done := make(chan error, 1)
	s.kafka.Produce(ctx, rec, func(_ *kgo.Record, err error) {
		done <- err
	})
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func maybeGunzip(rc io.ReadCloser, contentEncoding string) (io.ReadCloser, error) {
	if strings.EqualFold(strings.TrimSpace(contentEncoding), "gzip") {
		gr, err := gzip.NewReader(rc)
		if err != nil {
			_ = rc.Close()
			return nil, err
		}
		return readCloser{
			Reader: gr,
			closeFn: func() error {
				_ = gr.Close()
				return rc.Close()
			},
		}, nil
	}
	return rc, nil
}

type readCloser struct {
	io.Reader
	closeFn func() error
}

func (r readCloser) Close() error {
	if r.closeFn == nil {
		return errors.New("no closeFn")
	}
	return r.closeFn()
}

func newULID() string {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}