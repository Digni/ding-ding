package notifier

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Digni/ding-ding/internal/config"
)

func sendWebhook(cfg config.WebhookConfig, msg Message) error {
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	method := cfg.Method
	if method == "" {
		method = "POST"
	}

	req, err := http.NewRequest(method, cfg.URL, strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	return nil
}
