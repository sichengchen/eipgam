package provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"strings"
	"time"
)

// What the plan readers that sign with an account's access key share
// (tencent_usage.go, alibaba_usage.go).

// replyHead is the start of a reply magpie couldn't read, to say what it
// was in an error.
func replyHead(raw []byte) string {
	s := strings.TrimSpace(QuotedReply(raw))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

func hmacSHA256(key []byte, s string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(s))
	return m.Sum(nil)
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// jsonNumber is a number a reply gives as a number or as a string.
func jsonNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(x), 64)
		return f, err == nil
	}
	return 0, false
}

// resetTime is a reset as a reply gives it: epoch seconds, milliseconds
// or RFC 3339; nil for none.
func resetTime(v any) *time.Time {
	if s, ok := v.(string); ok {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(s)); err == nil {
			return &t
		}
	}
	n, ok := jsonNumber(v)
	if !ok || n <= 0 {
		return nil
	}
	// 1e11 seconds is the year 5138, 1e11 ms 1973: no reset is on the
	// wrong side of it
	var t time.Time
	if n > 1e11 {
		t = time.UnixMilli(int64(n))
	} else {
		t = time.Unix(int64(n), 0)
	}
	return &t
}
