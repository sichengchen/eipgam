package sessions

import (
	"encoding/binary"
	"errors"
	"strings"
	"time"
)

// The Antigravity step records agy CLI writes, read from their protobuf
// bytes: moved here from the Antigravity app's reader, which was removed.

// stepTimes is how long a model's step took from its start: to its first
// content (ttft) and to its end (millis), in ms; 0 for what isn't known.
type stepTimes struct{ ttft, millis int64 }

// stepTimesOf reads a step's times from its metadata: 1 its start, 6 its
// first content, 7 its model's end. A time before the start, or a first
// content after the end, is not known.
func stepTimesOf(b []byte) stepTimes {
	var start, first, end time.Time
	_ = pbWalk(b, func(num int, v uint64, d []byte) {
		if d == nil {
			return
		}
		switch num {
		case 1:
			start = pbTime(d)
		case 6:
			first = pbTime(d)
		case 7:
			end = pbTime(d)
		}
	})
	return timesFrom(start, first, end)
}

// agAppModel is the model a generator's message names: its field 19, else
// 21, else 22.
func agAppModel(b []byte) string {
	var named [3]string
	_ = pbWalk(b, func(num int, v uint64, d []byte) {
		if num != 1 || d == nil {
			return
		}
		_ = pbWalk(d, func(n int, v uint64, d []byte) {
			if d == nil {
				return
			}
			switch n {
			case 19:
				named[0] = strings.TrimSpace(string(d))
			case 21:
				named[1] = strings.TrimSpace(string(d))
			case 22:
				named[2] = strings.TrimSpace(string(d))
			}
		})
	})
	for _, m := range named {
		if m != "" && len(m) <= 200 {
			if base, ok := GeminiDeployment(m); ok {
				return base
			}
			return m
		}
	}
	return ""
}

// pbWalk walks a protobuf message's fields, handing each to field: a
// varint's value, or a length-delimited field's bytes (non-nil). Fixed-width
// fields are skipped.
func pbWalk(b []byte, field func(num int, v uint64, d []byte)) error {
	for len(b) > 0 {
		tag, n := binary.Uvarint(b)
		if n <= 0 || tag>>3 == 0 {
			return errAgApp
		}
		b = b[n:]
		num := int(tag >> 3)
		switch tag & 7 {
		case 0:
			v, n := binary.Uvarint(b)
			if n <= 0 {
				return errAgApp
			}
			b = b[n:]
			field(num, v, nil)
		case 1:
			if len(b) < 8 {
				return errAgApp
			}
			b = b[8:]
		case 2:
			l, n := binary.Uvarint(b)
			if n <= 0 || l > uint64(len(b)-n) {
				return errAgApp
			}
			field(num, 0, b[n:n+int(l)])
			b = b[n+int(l):]
		case 5:
			if len(b) < 4 {
				return errAgApp
			}
			b = b[4:]
		default:
			return errAgApp
		}
	}
	return nil
}

// timesFrom is stepTimes from a call's start, first content and end.
func timesFrom(start, first, end time.Time) stepTimes {
	var t stepTimes
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return t
	}
	t.millis = max(end.Sub(start).Milliseconds(), 1)
	if !first.IsZero() && !first.Before(start) && !first.After(end) {
		t.ttft = max(first.Sub(start).Milliseconds(), 1)
	}
	return t
}

// errAgApp is a protobuf that can't be read.
var errAgApp = errors.New("antigravity: unreadable metadata")

// GeminiDeployment is the Gemini model a Google deployment's name stands
// for, and whether m is such a name: a gemini-* id with one letter after
// its last hyphen. Google names the deployment serving gemini-3.8-flash
// "gemini-3.8-flash-n" (the owner's real Antigravity account, b2432754e)
// and the one serving gemini-4-argon "gemini-4-argon-d" (oioio's
// Antigravity database, 11-06): ids no request can ask for, which are that
// model, at its price. No Gemini model's own id ends in a lone letter
// (-lite, -nano, -pro, -001 and -exp are longer), and other makers' ids
// are left as written.
func GeminiDeployment(m string) (string, bool) {
	i := strings.LastIndexByte(m, '-')
	if i < 0 || len(m)-i != 2 || m[i+1] < 'a' || m[i+1] > 'z' {
		return m, false
	}
	base := m[:i]
	if !strings.HasPrefix(base, "gemini-") || len(base) == len("gemini-") {
		return m, false
	}
	return base, true
}

// pbTime is a google.protobuf.Timestamp, zero when it has no seconds.
func pbTime(b []byte) time.Time {
	var sec, nanos uint64
	_ = pbWalk(b, func(num int, v uint64, d []byte) {
		switch {
		case d != nil:
		case num == 1:
			sec = v
		case num == 2:
			nanos = v
		}
	})
	if sec == 0 || sec > 1<<40 || nanos >= 1e9 {
		return time.Time{}
	}
	return time.Unix(int64(sec), int64(nanos))
}
