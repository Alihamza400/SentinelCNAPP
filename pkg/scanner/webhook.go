package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
)

// Topic constants for scanner events.
const (
	TopicScanTriggered = "sentinel.scan.triggered"
)

// ErrPartialEmit is returned when some findings in a batch failed to emit.
var ErrPartialEmit = fmt.Errorf("some findings failed to emit")

// WebhookEvent represents a generic webhook event from a Git provider.
type WebhookEvent struct {
	Provider    string `json:"provider"`    // "github", "gitlab"
	EventType   string `json:"event_type"`  // "push", "pull_request"
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	CommitSHA   string `json:"commit_sha"`
	CommitMsg   string `json:"commit_msg"`
	PusherName  string `json:"pusher_name"`
	PusherEmail string `json:"pusher_email"`
	Ref         string `json:"ref"`
	Before      string `json:"before"`
	After       string `json:"after"`
}

// WebhookReceiver handles incoming Git provider webhooks.
type WebhookReceiver struct {
	log       *logging.Logger
	secret    string
	handlers  map[string]WebhookHandler
}

// WebhookHandler processes a webhook event.
type WebhookHandler func(ctx context.Context, event *WebhookEvent) error

// NewWebhookReceiver creates a new webhook receiver.
func NewWebhookReceiver(log *logging.Logger, secret string) *WebhookReceiver {
	return &WebhookReceiver{
		log:      log,
		secret:   secret,
		handlers: make(map[string]WebhookHandler),
	}
}

// RegisterHandler registers a handler for a specific event type.
func (r *WebhookReceiver) RegisterHandler(eventType string, handler WebhookHandler) {
	r.handlers[eventType] = handler
}

// Handler returns an HTTP handler for webhook endpoints.
func (r *WebhookReceiver) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		// Determine provider from User-Agent or header
		provider := req.Header.Get("X-GitHub-Event")
		if provider != "" {
			r.handleGitHub(req.Context(), string(body), provider, w)
			return
		}

		provider = req.Header.Get("X-Gitlab-Event")
		if provider != "" {
			r.handleGitLab(req.Context(), string(body), provider, w)
			return
		}

		http.Error(w, "unknown provider", http.StatusBadRequest)
	}
}

func (r *WebhookReceiver) handleGitHub(ctx context.Context, body string, eventType string, w http.ResponseWriter) {
	r.log.Info("received GitHub webhook", "event", eventType)

	// Parse GitHub push event
	var push struct {
		Ref        string `json:"ref"`
		Before     string `json:"before"`
		After      string `json:"after"`
		Repository struct {
			CloneURL string `json:"clone_url"`
			FullName string `json:"full_name"`
		} `json:"repository"`
		HeadCommit struct {
			ID      string `json:"id"`
			Message string `json:"message"`
		} `json:"head_commit"`
		Pusher struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"pusher"`
	}

	if err := json.Unmarshal([]byte(body), &push); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	event := &WebhookEvent{
		Provider:    "github",
		EventType:   eventType,
		RepoURL:     push.Repository.CloneURL,
		Branch:      push.Ref,
		CommitSHA:   push.HeadCommit.ID,
		CommitMsg:   push.HeadCommit.Message,
		PusherName:  push.Pusher.Name,
		PusherEmail: push.Pusher.Email,
		Ref:         push.Ref,
		Before:      push.Before,
		After:       push.After,
	}

	if handler, ok := r.handlers[eventType]; ok {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := handler(ctx, event); err != nil {
				r.log.Error("webhook handler failed", err, "event", eventType)
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func (r *WebhookReceiver) handleGitLab(ctx context.Context, body string, eventType string, w http.ResponseWriter) {
	r.log.Info("received GitLab webhook", "event", eventType)
	// TODO: Phase 3 - Implement GitLab webhook parsing
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}
