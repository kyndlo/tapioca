package control

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"
)

type Server struct {
	input   io.Reader
	output  io.Writer
	handler *Handler

	maxConcurrency int
	admission      chan struct{}
	writeMu        sync.Mutex
	mutatingMu     sync.Mutex
	seenMu         sync.Mutex
	seenRequests   map[string]struct{}
}

const DefaultMaxConcurrency = 8

func NewServer(input io.Reader, output io.Writer, handler *Handler) *Server {
	return NewServerWithOptions(input, output, handler, ServerOptions{})
}

type ServerOptions struct {
	MaxConcurrency int
}

func NewServerWithOptions(
	input io.Reader,
	output io.Writer,
	handler *Handler,
	options ServerOptions,
) *Server {
	if handler == nil {
		handler = NewHandler(Dependencies{})
	}
	if options.MaxConcurrency <= 0 {
		options.MaxConcurrency = DefaultMaxConcurrency
	}
	return &Server{
		input:          input,
		output:         output,
		handler:        handler,
		maxConcurrency: options.MaxConcurrency,
		admission:      make(chan struct{}, options.MaxConcurrency),
		seenRequests:   make(map[string]struct{}),
	}
}

func (s *Server) Run(ctx context.Context) error {
	sessionContext, cancelSession := context.WithCancel(ctx)
	defer cancelSession()
	defer s.handler.dependencies.Servers.Close()

	scanner := bufio.NewScanner(s.input)
	scanner.Buffer(make([]byte, 64*1024), MaxRequestBytes+1)

	var workers sync.WaitGroup
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		request, requestError := DecodeRequest(line)
		if requestError != nil {
			if err := s.write(Failure(request, requestError)); err != nil {
				return err
			}
			continue
		}
		if !s.markRequestSeen(request.ID) {
			if err := s.write(Failure(request, &ProtocolError{
				Code:      "duplicate_request",
				Message:   "request ID has already been used",
				Retryable: false,
				Details:   map[string]any{"id": request.ID},
			})); err != nil {
				return err
			}
			continue
		}

		// Cancellation must remain available even when all regular admission
		// slots are occupied, and its handler is short and non-blocking.
		if request.Method == "job.cancel" {
			s.process(sessionContext, request)
			continue
		}
		select {
		case s.admission <- struct{}{}:
		default:
			if err := s.write(Failure(request, &ProtocolError{
				Code:      "server_busy",
				Message:   "the control plane is at its concurrency limit",
				Retryable: true,
				Details: map[string]any{
					"max_concurrency": s.maxConcurrency,
				},
			})); err != nil {
				return err
			}
			continue
		}

		workers.Add(1)
		go func() {
			defer workers.Done()
			defer func() { <-s.admission }()
			s.process(sessionContext, request)
		}()
	}
	if err := scanner.Err(); err != nil {
		cancelSession()
		workers.Wait()
		return err
	}
	cancelSession()
	workers.Wait()
	return nil
}

func (s *Server) process(ctx context.Context, request Request) {
	if isMutating(request.Method) {
		s.mutatingMu.Lock()
		defer s.mutatingMu.Unlock()
	}

	emitter := &jobEmitter{
		server: s, jobID: request.JobID, now: s.handler.dependencies.Now,
	}
	if request.JobID != "" && request.Method != "job.cancel" {
		if err := emitter.emit(EventJobStarted, map[string]any{
			"request_id": request.ID,
			"method":     request.Method,
		}); err != nil {
			return
		}
		ctx = context.WithValue(ctx, reporterContextKey{}, emitter)
	}

	result, requestError := s.handler.Handle(ctx, request)
	if requestError != nil {
		if err := s.write(Failure(request, requestError)); err != nil {
			return
		}
		if request.JobID != "" && request.Method != "job.cancel" {
			_ = emitter.emit(EventJobFailed, map[string]any{
				"request_id": request.ID,
				"error":      requestError,
			})
		}
		return
	}

	if err := s.write(Success(request, result)); err != nil {
		return
	}
	if request.JobID != "" && request.Method != "job.cancel" {
		_ = emitter.emit(EventJobCompleted, map[string]any{
			"request_id": request.ID,
		})
	}
}

const MaxEventDataBytes = 16 * 1024

type reporterContextKey struct{}

type jobEmitter struct {
	server *Server
	jobID  string
	now    func() time.Time
	mu     sync.Mutex
	seq    uint64
}

func (e *jobEmitter) emit(name EventName, data any) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.seq++
	return e.server.write(NewEvent(e.jobID, name, e.seq, e.now(), boundEventData(data)))
}

func reportProgress(ctx context.Context, data any) {
	if emitter, ok := ctx.Value(reporterContextKey{}).(*jobEmitter); ok {
		_ = emitter.emit(EventJobProgress, data)
	}
}

func reportLog(ctx context.Context, message string) {
	if len(message) > 4096 {
		message = message[:4096]
	}
	if emitter, ok := ctx.Value(reporterContextKey{}).(*jobEmitter); ok {
		_ = emitter.emit(EventJobLog, map[string]any{"message": message})
	}
}

func boundEventData(data any) any {
	encoded, err := json.Marshal(data)
	if err == nil && len(encoded) <= MaxEventDataBytes {
		return data
	}
	return map[string]any{
		"truncated": true,
		"message":   "event payload exceeded 16 KiB",
	}
}

func (s *Server) markRequestSeen(id string) bool {
	s.seenMu.Lock()
	defer s.seenMu.Unlock()
	if _, exists := s.seenRequests[id]; exists {
		return false
	}
	s.seenRequests[id] = struct{}{}
	return true
}

func isMutating(method string) bool {
	switch method {
	case "model.pull", "model.remove", "server.start", "server.stop",
		"image.generate", "video.generate", "speech.generate", "voice.clone":
		return true
	default:
		return false
	}
}

func (s *Server) write(envelope any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if err := json.NewEncoder(s.output).Encode(envelope); err != nil {
		return err
	}
	if flusher, ok := s.output.(interface{ Flush() error }); ok {
		return flusher.Flush()
	}
	return nil
}
