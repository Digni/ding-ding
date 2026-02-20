package notifier

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

func systemNotify(title, body string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("notify-send", "--app-name=ding-ding", title, body).Run()
	case "darwin":
		script := `display notification (item 1 of argv) with title (item 2 of argv)`
		return exec.Command("osascript", "-e", "on run argv", "-e", script, "-e", "end run", body, title).Run()
	case "windows":
		escTitle := strings.ReplaceAll(xmlEscape(title), "%", "%%")
		escBody := strings.ReplaceAll(xmlEscape(body), "%", "%%")
		ps := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$template = '<toast><visual><binding template="ToastText02"><text id="1">%s</text><text id="2">%s</text></binding></visual></toast>'
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("ding-ding").Show($toast)
`, escTitle, escBody)
		return exec.Command("powershell", "-Command", ps).Run()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
