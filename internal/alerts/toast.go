package alerts

// toastScriptName is the PowerShell file Notify keeps under
// NotifyOptions.Dir on Windows. The text reaches it as parameters, never
// as script code.
const toastScriptName = "toast.ps1"

// toastScript shows a toast through the Windows Runtime notification API.
// It runs under the application id of Windows PowerShell, which every
// Windows installation registers in the Start menu, so the kit needs no
// registration of its own for the toast to appear.
const toastScript = `param([string]$Title, [string]$Body)
$null = [Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime]
$null = [Windows.Data.Xml.Dom.XmlDocument, Windows.Data.Xml.Dom.XmlDocument, ContentType = WindowsRuntime]
$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$texts = $template.GetElementsByTagName("text")
$null = $texts.Item(0).AppendChild($template.CreateTextNode($Title))
$null = $texts.Item(1).AppendChild($template.CreateTextNode($Body))
$toast = [Windows.UI.Notifications.ToastNotification]::new($template)
$source = "{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe"
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($source).Show($toast)
`
