package guard

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"unicode/utf16"
)

func hookCommand(bin string, agent Agent, strict bool, goos string) string {
	args := " guard check --agent " + string(agent)
	if strict {
		args += " --strict"
	}
	if goos == "windows" {
		return windowsHook(bin, args)
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`").Replace(bin) + `"` + args
}

// EncodedCommand has no shell metacharacters, so sh, Git Bash and PowerShell
// pass the same argv. UTF-16LE is required by Windows PowerShell 5.1.
func windowsHook(bin, args string) string {
	script := "$ErrorActionPreference = 'Stop'; " +
		"$OutputEncoding = [Console]::InputEncoding = [Console]::OutputEncoding = New-Object System.Text.UTF8Encoding; " +
		"try { [Console]::In.ReadToEnd() | & '" + strings.ReplaceAll(bin, "'", "''") + "'" + args +
		"; exit $LASTEXITCODE } catch { [Console]::Error.WriteLine($_); exit 2 }"
	units := utf16.Encode([]rune(script))
	data := make([]byte, len(units)*2)
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[i*2:], unit)
	}
	return "powershell.exe -NoProfile -NonInteractive -InputFormat Text -OutputFormat Text -EncodedCommand " +
		base64.StdEncoding.EncodeToString(data)
}

// HookBinary returns the binary path an installed hook command names, in
// the POSIX quoted form or the Windows encoded PowerShell form, so doctor
// can compare it with the running binary. False when the command is not
// one the kit writes.
func HookBinary(command string) (string, bool) {
	command = strings.TrimSpace(command)
	if strings.HasPrefix(command, `"`) {
		return posixHookBinary(command[1:])
	}
	if script, ok := decodeEncodedCommand(command); ok {
		return windowsHookBinary(script)
	}
	return "", false
}

// posixHookBinary reads the quoted path hookCommand wrote, undoing its
// escapes, up to the closing quote.
func posixHookBinary(rest string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(rest); i++ {
		switch c := rest[i]; {
		case c == '\\' && i+1 < len(rest):
			i++
			b.WriteByte(rest[i])
		case c == '"':
			return b.String(), b.Len() > 0
		default:
			b.WriteByte(c)
		}
	}
	return "", false
}

// decodeEncodedCommand returns the script of a powershell.exe
// -EncodedCommand invocation.
func decodeEncodedCommand(command string) (string, bool) {
	words := strings.Fields(command)
	for i := 0; i+1 < len(words); i++ {
		if !strings.EqualFold(words[i], "-EncodedCommand") {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(words[i+1])
		if err != nil || len(data)%2 != 0 {
			return "", false
		}
		units := make([]uint16, len(data)/2)
		for j := range units {
			units[j] = binary.LittleEndian.Uint16(data[j*2:])
		}
		return string(utf16.Decode(units)), true
	}
	return "", false
}

// windowsHookBinary reads the single-quoted path after the call operator
// of the decoded script, undoing the doubled apostrophes.
func windowsHookBinary(script string) (string, bool) {
	const call = "| & '"
	start := strings.Index(script, call)
	if start < 0 {
		return "", false
	}
	rest := script[start+len(call):]
	var b strings.Builder
	for i := 0; i < len(rest); i++ {
		if rest[i] != '\'' {
			b.WriteByte(rest[i])
			continue
		}
		if i+1 < len(rest) && rest[i+1] == '\'' {
			b.WriteByte('\'')
			i++
			continue
		}
		return b.String(), b.Len() > 0
	}
	return "", false
}
