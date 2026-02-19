package notifier

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Digni/ding-ding/internal/config"
)

func sendDiscord(cfg config.DiscordConfig, msg Message) error {
	content := fmt.Sprintf("**%s**\n%s", msg.Title, msg.Body)
	if msg.Agent != "" {
		content = fmt.Sprintf("**%s** (%s)\n%s", msg.Title, msg.Agent, msg.Body)
	}

	payload, err := json.Marshal(map[string]string{
		"content": content,
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	resp, err := http.Post(cfg.WebhookURL, "application/json", strings.NewReader(string(payload)))
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord returned status %d", resp.StatusCode)
	}

	return nil
}
