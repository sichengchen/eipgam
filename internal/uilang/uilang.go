// Package uilang is the language eipgam speaks in: the one the user picked
// in Settings (settings.Lang), or with "system" the system's. The GUI's
// page picks its own the same way (i18n.js setLocale and sysLocale); the
// tray, notices and the TUI ask Pick.
package uilang

import "strings"

// Pick is the language for the setting pref: English, whatever pref or
// the system says. This fork keeps no other language; the tray, notices
// and the TUI still ask here, so one can come back in one place.
func Pick(pref string, system func() string) string {
	return "en"
}

// Env is the language the environment names: the first of LC_ALL,
// LC_MESSAGES, LANG and LANGUAGE set to one.
func Env(getenv func(string) string) string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG", "LANGUAGE"} {
		if v := getenv(k); v != "" && v != "C" && v != "POSIX" && !strings.HasPrefix(v, "C.") {
			return v
		}
	}
	return ""
}

// firstAppleLang is the first entry `defaults read -g AppleLanguages`
// prints: ( "zh-Hans-CN", "en-US" ), one to a line, quoted or not.
func firstAppleLang(out string) string {
	for _, line := range strings.Split(out, "\n") {
		s := strings.Trim(strings.TrimSpace(line), `",`)
		if s != "" && s != "(" && s != ")" {
			return s
		}
	}
	return ""
}
