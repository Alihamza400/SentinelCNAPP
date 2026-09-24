package scanner

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
)

const (
	WebhookMaxBodySize = 10 * 1024 * 1024 // 10MB max webhook body size

	// TopicScanTriggered is the event topic for scan trigger requests.
	TopicScanTriggered = "sentinel.scan.triggered"
)

// WebhookEvent represents a generic webhook event from a Git provider.
type WebhookEvent struct {
	Provider    string `json:"provider"`   // "github", "gitlab"
	EventType   string `json:"event_type"` // "push", "pull_request"
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

// WebhookHandler processes a webhook event.
type WebhookHandler func(ctx context.Context, event *WebhookEvent) error

// WebhookReceiver handles incoming Git provider webhooks.
type WebhookReceiver struct {
	log      *logging.Logger
	secret   string
	handlers map[string]WebhookHandler
}

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

		body, err := io.ReadAll(io.LimitReader(req.Body, WebhookMaxBodySize))
		if err != nil {
			http.Error(w, "failed to read body", http.StatusBadRequest)
			return
		}
		defer req.Body.Close()

		// Verify HMAC signature when a secret is configured.
		if r.secret != "" {
			signature := req.Header.Get("X-Hub-Signature-256")
			if signature == "" {
				http.Error(w, "missing signature header", http.StatusBadRequest)
				return
			}
			valid, err := VerifyWebhookSignature(body, signature, r.secret)
			if err != nil {
				http.Error(w, fmt.Sprintf("signature verification error: %v", err), http.StatusBadRequest)
				return
			}
			if !valid {
				http.Error(w, "invalid signature", http.StatusUnauthorized)
				return
			}
		}

		// Determine provider from event header.
		if eventType := req.Header.Get("X-GitHub-Event"); eventType != "" {
			r.handleGitHub(req.Context(), body, eventType, w)
			return
		}
		if eventType := req.Header.Get("X-Gitlab-Event"); eventType != "" {
			r.handleGitLab(req.Context(), body, eventType, w)
			return
		}

		http.Error(w, "unknown provider", http.StatusBadRequest)
	}
}

func (r *WebhookReceiver) handleGitHub(ctx context.Context, body []byte, eventType string, w http.ResponseWriter) {
	r.log.Info("received GitHub webhook", "event", eventType)

	if eventType == "ping" {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "pong"})
		return
	}

	// Parse GitHub push event payload.
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

	if err := json.Unmarshal(body, &push); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	branch := strings.TrimPrefix(push.Ref, "refs/heads/")

	event := &WebhookEvent{
		Provider:    "github",
		EventType:   eventType,
		RepoURL:     push.Repository.CloneURL,
		Branch:      branch,
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
			hctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := handler(hctx, event); err != nil {
				r.log.Error("webhook handler failed", err, "event", eventType)
			}
		}()
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func (r *WebhookReceiver) handleGitLab(ctx context.Context, body []byte, eventType string, w http.ResponseWriter) {
	r.log.Info("received GitLab webhook", "event", eventType)

	// GitLab push hook payload.
	if strings.Contains(eventType, "Push Hook") || eventType == "push" {
		var push struct {
			Ref     string `json:"ref"`
			Before  string `json:"before"`
			After   string `json:"after"`
			Project struct {
				GitHTTPURL string `json:"git_http_url"`
				PathWithNamespace string `json:"path_with_namespace"`
			} `json:"project"`
			Commits []struct {
				ID    string `json:"id"`
				Message string `json:"message"`
			} `json:"commits"`
			UserUsername string `json:"user_username"`
			UserEmail    string `json:"user_email"`
		}

		if err := json.Unmarshal(body, &push); err != nil {
			http.Error(w, "invalid payload", http.StatusBadRequest)
			return
		}

		branch := strings.TrimPrefix(push.Ref, "refs/heads/")
		commitSHA := push.After
		commitMsg := ""
		if len(push.Commits) > 0 {
			commitSHA = push.Commits[len(push.Commits)-1].ID
			commitMsg = push.Commits[len(push.Commits)-1].Message
		}

		event := &WebhookEvent{
			Provider:    "gitlab",
			EventType:   "push",
			RepoURL:     push.Project.GitHTTPURL,
			Branch:      branch,
			CommitSHA:   commitSHA,
			CommitMsg:   commitMsg,
			PusherName:  push.UserUsername,
			PusherEmail: push.UserEmail,
			Ref:         push.Ref,
			Before:      push.Before,
			After:       push.After,
		}

		if handler, ok := r.handlers["push"]; ok {
			go func() {
				hctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
				defer cancel()
				if err := handler(hctx, event); err != nil {
					r.log.Error("webhook handler failed", err, "event", "push")
				}
			}()
		}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func VerifyWebhookSignature(body []byte, signature string, secret string) (bool, error) {
	if secret == "" {
		return true, nil
	}

	if !strings.HasPrefix(signature, "sha256=") {
		return false, fmt.Errorf("invalid signature format: must start with sha256=")
	}
		sigHex := strings.TrimPrefix(signature, "sha256=")
	
decodedSig, err := hex.DecodeString(sigHex)
	if err != nil {
		return false, fmt.Errorf("invalid hex encoding in signature: %w", err)
	}

	hmacObj := hmac.New(sha256.New, []byte(secret))
	hmacObj.Write(body)
	computedHash := hmacObj.Sum(nil)

	if !hmac.Equal(decodedSig, computedHash) {
		return false, nil
	}

	return true, nil
}

func HandleWebhookWithSizeLimit(next http.HandlerFunc, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Check content length before reading body
		if r.ContentLength > WebhookMaxBodySize {
			http.Error(w, "webhook payload too large", http.StatusRequestEntityTooLarge)
			return
		}

		// Read body with size limit
		body, err := io.ReadAll(io.LimitReader(r.Body, WebhookMaxBodySize))
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))

		// Get signature from header
		signature := r.Header.Get("X-Hub-Signature-256")
		if signature == "" {
			http.Error(w, "missing signature header", http.StatusBadRequest)
			return
		}

		// Verify signature
		valid, err := VerifyWebhookSignature(body, signature, secret)
		if err != nil {
			http.Error(w, fmt.Sprintf("signature verification error: %v", err), http.StatusBadRequest)
			return
		}
		if !valid {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}
