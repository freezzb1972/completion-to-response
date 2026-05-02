package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/NoahStepheno/completion-to-response/internal/config"
	"github.com/NoahStepheno/completion-to-response/internal/types"
	"github.com/NoahStepheno/completion-to-response/internal/transform"
)

type server struct {
	cfg    *config.Config
	client *http.Client
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func main() {
	cfg := config.Parse()

	if cfg.BackendURL == "" {
		log.Fatal("Error: -url flag is required (backend chat completions endpoint)")
	}

	if cfg.LogFile != "" {
		f, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("Error: cannot open log file: %v", err)
		}
		defer f.Close()
		log.SetOutput(f)
		log.Printf("Logging to file: %s", cfg.LogFile)
	}

	srv := &server{
		cfg: cfg,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", srv.handleHealth)
	mux.HandleFunc("/responses", srv.withLogging(srv.withRecovery(srv.handleResponses)))
	mux.HandleFunc("/v1/responses", srv.withLogging(srv.withRecovery(srv.handleResponses)))

	httpAddr := ":" + cfg.Port

	log.Printf("Starting server on port %s", cfg.Port)
	log.Printf("Backend URL: %s", cfg.BackendURL)
	log.Printf("Default model: %s", cfg.DefaultModel)

	server := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	shutdown(server)
}

func shutdown(srv *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *server) handleResponses(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read raw request body
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		respondError(w, fmt.Sprintf("failed to read request: %v", err), http.StatusBadRequest)
		return
	}
	log.Printf("[DEBUG] Incoming request: %s", trunc(prettyJSON(reqBody), 2048))

	var req types.ResponsesAPIRequest
	if err := json.Unmarshal(reqBody, &req); err != nil {
		respondError(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	compReq := transform.RequestToCompletion(req)

	if s.cfg.ForceModel != "" {
		compReq.Model = s.cfg.ForceModel
	} else if compReq.Model == "" {
		compReq.Model = s.cfg.DefaultModel
	}

	compBody, err := json.Marshal(compReq)
	if err != nil {
		respondError(w, fmt.Sprintf("failed to marshal request: %v", err), http.StatusInternalServerError)
		return
	}

	apiKey := s.extractAPIKey(r)
	if apiKey == "" {
		respondError(w, "missing API key: set Authorization header or -key flag", http.StatusUnauthorized)
		return
	}

	log.Printf("[DEBUG] Forwarding to backend (%s): %s", s.cfg.BackendURL, trunc(prettyJSON(compBody), 4096))

	if req.Stream {
		s.handleStream(w, r, compReq)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.cfg.Timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BackendURL, bytes.NewReader(compBody))
	if err != nil {
		respondError(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Authorization", "Bearer "+s.extractAPIKey(r))
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		respondError(w, fmt.Sprintf("backend request failed: %v", err), http.StatusBadGateway)
		return
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		respondError(w, fmt.Sprintf("failed to read response: %v", err), http.StatusBadGateway)
		return
	}
	log.Printf("[DEBUG] Backend response (status %d): %s", httpResp.StatusCode, prettyJSON(respBody))

	if httpResp.StatusCode >= 400 {
		respondError(w, fmt.Sprintf("backend error: %s", respBody), httpResp.StatusCode)
		return
	}

	var compResp types.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &compResp); err != nil {
		respondError(w, fmt.Sprintf("failed to unmarshal response: %v", err), http.StatusBadGateway)
		return
	}

	resp := transform.ResponseFromCompletion(compResp)

	respJSON, _ := json.Marshal(resp)
	log.Printf("[DEBUG] Transformed response to client: %s", prettyJSON(respJSON))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respJSON)
}

func (s *server) handleStream(w http.ResponseWriter, r *http.Request, compReq types.ChatCompletionRequest) {
	log.Printf("[DEBUG] handleStream: started")

	body, err := json.Marshal(compReq)
	if err != nil {
		log.Printf("[DEBUG] handleStream: marshal error: %v", err)
		respondError(w, fmt.Sprintf("failed to marshal request: %v", err), http.StatusInternalServerError)
		return
	}

	httpReq, err := http.NewRequestWithContext(r.Context(), http.MethodPost, s.cfg.BackendURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[DEBUG] handleStream: new request error: %v", err)
		respondError(w, fmt.Sprintf("failed to create request: %v", err), http.StatusInternalServerError)
		return
	}

	httpReq.Header.Set("Authorization", "Bearer "+s.extractAPIKey(r))
	httpReq.Header.Set("Content-Type", "application/json")

	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("[DEBUG] handleStream: flusher not supported")
		respondError(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	log.Printf("[DEBUG] handleStream: calling backend %s", s.cfg.BackendURL)
	httpResp, err := s.client.Do(httpReq)
	if err != nil {
		log.Printf("[DEBUG] handleStream: backend call failed: %v", err)
		respondError(w, fmt.Sprintf("backend request failed: %v", err), http.StatusBadGateway)
		return
	}
	defer httpResp.Body.Close()

	log.Printf("[DEBUG] handleStream: backend responded with status %d", httpResp.StatusCode)

	if httpResp.StatusCode >= 400 {
		errBody, _ := io.ReadAll(httpResp.Body)
		log.Printf("[DEBUG] Backend stream error (status %d): %s", httpResp.StatusCode, string(errBody))
		respondError(w, fmt.Sprintf("backend error: %s", errBody), httpResp.StatusCode)
		return
	}

	converter := transform.NewStreamConverter(compReq.Model)

	// Parse SSE stream from backend: "data: {json}\n\n" lines
	scanner := bufio.NewScanner(httpResp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()

		if line == "" || !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk types.ChatCompletionStreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			log.Printf("[DEBUG] SSE parse error: %v, raw: %s", err, data)
			continue
		}

		events := converter.OnChunk(chunk)
		for _, eventData := range events {
			fmt.Fprintf(w, "data: %s\n\n", eventData)
		}
		flusher.Flush()
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[DEBUG] SSE scanner error: %v", err)
	}
	log.Printf("[DEBUG] handleStream: finished")
}

func (s *server) withLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		rw := &responseWriter{ResponseWriter: w}

		next(rw, r)

		log.Printf("%s %s %d %v", r.Method, r.URL.Path, rw.status, time.Since(start))
	}
}

func (s *server) withRecovery(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rc := recover(); rc != nil {
				log.Printf("Panic recovered: %v", rc)
				respondError(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next(w, r)
	}
}

func respondError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func (s *server) extractAPIKey(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return s.cfg.APIKey
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func prettyJSON(data []byte) string {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") != nil {
		return string(data)
	}
	return buf.String()
}

func trunc(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d bytes truncated)", len(s)-maxLen)
}
