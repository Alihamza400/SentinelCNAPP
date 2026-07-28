package scanner

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
)

const (
	WebhookMaxBodySize = 10 * 1024 * 1024 // 10MB max webhook body size
)

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
