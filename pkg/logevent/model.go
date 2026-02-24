package logevent

import "time"

type Source struct {
	Service  string `json:"service"`
	Instance string `json:"instance,omitempty"`
	Env      string `json:"env,omitempty"`
	Region   string `json:"region,omitempty"`
}

type Trace struct {
	TraceID   string `json:"trace_id,omitempty"`
	SpanID    string `json:"span_id,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

type LogEvent struct {
	EventID       string         `json:"event_id"`
	TenantID      string         `json:"tenant_id"`
	Source        Source         `json:"source"`
	ObservedAt    time.Time      `json:"observed_at"`
	EmittedAt     *time.Time     `json:"emitted_at,omitempty"`
	Level         string         `json:"level"`
	Trace         Trace          `json:"trace,omitempty"`
	SchemaVersion int            `json:"schema_version"`
	Body          map[string]any `json:"body"`
}