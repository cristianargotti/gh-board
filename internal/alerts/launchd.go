package alerts

import (
	"bytes"
	"context"
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Where macOS keeps the per-user agents and their logs.
const (
	launchAgentsDir = "Library/LaunchAgents"
	launchLogsDir   = "Library/Logs"
)

// The fixed parts of the agent plist.
const (
	plistHeader = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`
	plistFooter = "</dict>\n</plist>\n"
)

// plistPath is the agent file under the home.
func (j *job) plistPath() string {
	return filepath.Join(j.opts.Home, launchAgentsDir, jobName+".plist")
}

// logPath is where launchd sends the output of the job.
func (j *job) logPath() string {
	return filepath.Join(j.opts.Home, launchLogsDir, jobName+".log")
}

// launchdDomain is the per-user domain the agent is loaded into.
func (j *job) launchdDomain() string {
	uid := j.opts.UID
	if uid <= 0 {
		uid = os.Getuid()
	}
	return "gui/" + strconv.Itoa(uid)
}

// launchctl builds a launchctl command.
func launchctl(args ...string) Command {
	return Command{Name: "launchctl", Args: args}
}

// installLaunchd writes the agent and loads it. The bootout before the
// bootstrap lets a second install replace a loaded job; it fails when the
// job is not loaded, which is the common case, so its error is not one.
func (j *job) installLaunchd(ctx context.Context) error {
	path := j.plistPath()
	if err := j.write(path, j.plist()); err != nil {
		return err
	}
	_, _ = j.run(ctx, launchctl("bootout", j.launchdDomain()+"/"+jobName))
	_, err := j.run(ctx, launchctl("bootstrap", j.launchdDomain(), path))
	return err
}

// uninstallLaunchd unloads the agent and removes its file. The log the
// job wrote stays, because install did not write it.
func (j *job) uninstallLaunchd(ctx context.Context) error {
	path := j.plistPath()
	if !exists(path) {
		j.say("nothing to remove: %s is absent\n", path)
		return nil
	}
	_, _ = j.run(ctx, launchctl("bootout", j.launchdDomain()+"/"+jobName))
	return j.remove(path)
}

// plist renders the agent: the job as an argument vector, the interval in
// seconds, a run at load so that alerts.json appears right after install,
// the gh path for the keyring token and a log under Library/Logs.
func (j *job) plist() string {
	var b strings.Builder
	b.WriteString(plistHeader)
	plistKey(&b, "Label", "<string>"+jobName+"</string>")
	b.WriteString("\t<key>ProgramArguments</key>\n\t<array>\n")
	for _, arg := range j.command() {
		b.WriteString("\t\t<string>" + xmlEscape(arg) + "</string>\n")
	}
	b.WriteString("\t</array>\n")
	plistKey(&b, "StartInterval", "<integer>"+strconv.Itoa(int(j.opts.Interval.Seconds()))+"</integer>")
	plistKey(&b, "RunAtLoad", "<true/>")
	if j.ghPath != "" {
		b.WriteString("\t<key>EnvironmentVariables</key>\n\t<dict>\n")
		b.WriteString("\t\t<key>" + envGHPath + "</key>\n\t\t<string>" + xmlEscape(j.ghPath) + "</string>\n")
		b.WriteString("\t</dict>\n")
	}
	log := "<string>" + xmlEscape(j.logPath()) + "</string>"
	plistKey(&b, "StandardOutPath", log)
	plistKey(&b, "StandardErrorPath", log)
	b.WriteString(plistFooter)
	return b.String()
}

// plistKey writes one key with its already rendered value.
func plistKey(b *strings.Builder, key, value string) {
	b.WriteString("\t<key>" + key + "</key>\n\t" + value + "\n")
}

// xmlEscape makes a path or an argument safe inside a plist string.
func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s)) // a bytes.Buffer never fails
	return b.String()
}
