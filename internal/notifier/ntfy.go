package notifier

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Digni/ding-ding/internal/config"
)

func sendNtfy(cfg config.NtfyConfig, msg Message) error {
	url := fmt.Sprintf("%s/%s", strings.TrimRight(cfg.Server, "/"), cfg.Topic)

	req, err := http.NewRequest("POST", url, strings.NewReader(msg.Body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Title", msg.Title)

	if cfg.Priority != "" {
		req.Header.Set("Priority", cfg.Priority)
	}

	if msg.Agent != "" {
		req.Header.Set("Tags", msg.Agent)
	}

	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ntfy returned status %d", resp.StatusCode)
	}

	return nil
}
