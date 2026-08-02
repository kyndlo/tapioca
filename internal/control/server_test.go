package control

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestServerWritesResponsesAndJobEvents(t *testing.T) {
	input := strings.NewReader(
		"{\"version\":1,\"type\":\"request\",\"id\":\"req-1\",\"method\":\"health.get\",\"job_id\":\"job-1\"}\n",
	)
	var output bytes.Buffer
	server := NewServer(input, &output, NewHandler(Dependencies{}))
	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("output has %d lines, want 3:\n%s", len(lines), output.String())
	}
	schema := loadContractSchema(t)

	var types []string
	var events []string
	for _, line := range lines {
		if err := validateEnvelope(schema, []byte(line)); err != nil {
			t.Fatalf("server emitted nonconforming envelope: %v\n%s", err, line)
		}
		var envelope struct {
			Version  int    `json:"version"`
			Type     string `json:"type"`
			Event    string `json:"event"`
			Sequence uint64 `json:"sequence"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("invalid output JSON %q: %v", line, err)
		}
		if envelope.Version != ProtocolVersion {
			t.Fatalf("version = %q", envelope.Version)
		}
		types = append(types, envelope.Type)
		if envelope.Event != "" {
			events = append(events, envelope.Event)
		}
	}
	if strings.Join(types, ",") != "event,response,event" {
		t.Fatalf("envelope types = %v", types)
	}
	if strings.Join(events, ",") != "job.started,job.completed" {
		t.Fatalf("events = %v", events)
	}
	var started Event
	var completed Event
	if err := json.Unmarshal([]byte(lines[0]), &started); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[2]), &completed); err != nil {
		t.Fatal(err)
	}
	if started.Sequence != 1 || completed.Sequence != 2 ||
		started.Timestamp == "" || completed.Timestamp == "" {
		t.Fatalf("event ordering = %#v then %#v", started, completed)
	}
}

func TestServerWritesMalformedRequestError(t *testing.T) {
	input := strings.NewReader(
		"{not-json}\n" +
			"{\"version\":1,\"type\":\"request\",\"id\":\"req-2\",\"method\":\"health.get\"}\n",
	)
	var output bytes.Buffer
	if err := NewServer(input, &output, nil).Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("output has %d lines, want malformed error and recovered response", len(lines))
	}
	var malformed Response
	if err := json.Unmarshal([]byte(lines[0]), &malformed); err != nil {
		t.Fatalf("invalid malformed response JSON: %v", err)
	}
	if malformed.Error == nil || malformed.Error.Code != "invalid_request" {
		t.Fatalf("malformed response = %#v", malformed)
	}
	if malformed.ID != "" {
		t.Fatalf("malformed response id = %q, want empty uncorrelated id", malformed.ID)
	}
	schema := loadContractSchema(t)
	if err := validateEnvelope(schema, []byte(lines[0])); err != nil {
		t.Fatalf("malformed response violates contract: %v\n%s", err, lines[0])
	}
	if err := validateEnvelope(schema, []byte(lines[1])); err != nil {
		t.Fatalf("recovery response violates contract: %v\n%s", err, lines[1])
	}
	var recovered Response
	if err := json.Unmarshal([]byte(lines[1]), &recovered); err != nil {
		t.Fatalf("invalid recovered response JSON: %v", err)
	}
	if recovered.ID != "req-2" || recovered.Error != nil {
		t.Fatalf("recovered response = %#v", recovered)
	}
}

func TestServerFlushesEachEnvelope(t *testing.T) {
	var destination bytes.Buffer
	output := &countingFlusher{Writer: bufio.NewWriter(&destination)}
	input := strings.NewReader(
		"{\"version\":1,\"type\":\"request\",\"id\":\"req-1\",\"method\":\"health.get\"}\n",
	)
	if err := NewServer(input, output, nil).Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if output.flushes != 1 {
		t.Fatalf("flush count = %d, want 1", output.flushes)
	}
	if destination.Len() == 0 {
		t.Fatal("destination is empty after flush")
	}
}

func TestServerCancelsJobsAtEOF(t *testing.T) {
	handler := NewHandler(Dependencies{
		Catalog: func(ctx context.Context) ([]CatalogModel, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	input := strings.NewReader(
		"{\"version\":1,\"type\":\"request\",\"id\":\"req-1\",\"method\":\"catalog.list\",\"job_id\":\"job-1\"}\n",
	)
	var output bytes.Buffer
	if err := NewServer(input, &output, handler).Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(output.String(), `"code":"job_cancelled"`) {
		t.Fatalf("output does not contain cancellation error:\n%s", output.String())
	}
}

func TestServerRejectsDuplicateRequestIDs(t *testing.T) {
	input := strings.NewReader(
		"{\"version\":1,\"type\":\"request\",\"id\":\"same\",\"method\":\"health.get\"}\n" +
			"{\"version\":1,\"type\":\"request\",\"id\":\"same\",\"method\":\"health.get\"}\n",
	)
	var output bytes.Buffer
	if err := NewServer(input, &output, nil).Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if strings.Count(output.String(), `"code":"duplicate_request"`) != 1 {
		t.Fatalf("duplicate response count is not 1:\n%s", output.String())
	}
}

func TestServerBoundsAdmissionAndConcurrency(t *testing.T) {
	var active atomic.Int32
	var maximum atomic.Int32
	var once sync.Once
	started := make(chan struct{})
	handler := NewHandler(Dependencies{
		Catalog: func(ctx context.Context) ([]CatalogModel, error) {
			current := active.Add(1)
			defer active.Add(-1)
			for {
				previous := maximum.Load()
				if current <= previous || maximum.CompareAndSwap(previous, current) {
					break
				}
			}
			once.Do(func() { close(started) })
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	input := strings.NewReader(
		"{\"version\":1,\"type\":\"request\",\"id\":\"one\",\"method\":\"catalog.list\",\"job_id\":\"job-one\"}\n" +
			"{\"version\":1,\"type\":\"request\",\"id\":\"two\",\"method\":\"catalog.list\",\"job_id\":\"job-two\"}\n" +
			"{\"version\":1,\"type\":\"request\",\"id\":\"three\",\"method\":\"catalog.list\",\"job_id\":\"job-three\"}\n",
	)
	var output bytes.Buffer
	server := NewServerWithOptions(input, &output, handler, ServerOptions{MaxConcurrency: 1})
	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	select {
	case <-started:
	default:
		t.Fatal("admitted job did not start")
	}
	if maximum.Load() > 1 {
		t.Fatalf("maximum concurrency = %d, want at most 1", maximum.Load())
	}
	if strings.Count(output.String(), `"code":"server_busy"`) != 2 {
		t.Fatalf("server_busy count is not 2:\n%s", output.String())
	}
	if !strings.Contains(output.String(), `"retryable":true`) {
		t.Fatalf("busy error is not retryable:\n%s", output.String())
	}
}

func TestMutatingMethodClassification(t *testing.T) {
	if isMutating("job.cancel") {
		t.Fatal("job.cancel must bypass the mutator lock so it can stop a running job")
	}
	if isMutating("catalog.list") {
		t.Fatal("catalog.list must remain read-only")
	}
	if !isMutating("image.generate") || !isMutating("video.generate") {
		t.Fatal("image and video generation must use the one-GPU mutator lock")
	}
}

type countingFlusher struct {
	*bufio.Writer
	flushes int
}

func (f *countingFlusher) Flush() error {
	f.flushes++
	return f.Writer.Flush()
}
