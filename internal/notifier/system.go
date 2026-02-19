package notifier

import (
	"fmt"
	"os/exec"
	"runtime"
)

func systemNotify(title, body string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("notify-send", "--app-name=ding-ding", title, body).Run()
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		return exec.Command("osascript", "-e", script).Run()
	case "windows":
		// PowerShell toast notification
		ps := fmt.Sprintf(`
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null
[Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime] | Out-Null
$template = '<toast><visual><binding template="ToastText02"><text id="1">%s</text><text id="2">%s</text></binding></visual></toast>'
$xml = New-Object Windows.Data.Xml.Dom.XmlDocument
$xml.LoadXml($template)
$toast = [Windows.UI.Notifications.ToastNotification]::new($xml)
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier("ding-ding").Show($toast)
`, title, body)
		return exec.Command("powershell", "-Command", ps).Run()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
