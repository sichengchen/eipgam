package catalog

import "time"

// Peak names the hours a list price is the off-peak price of: a call made
// in them is billed at PeakFactor times it. DeepSeek bills its models at
// half price off-peak, and models.dev lists that half as the price, with a
// multiplier for the peak hours its api.json leaves out
// (providers/deepseek/models/deepseek-flash.toml: "Base prices below are
// off-peak; the multiplier doubles them at peak"). Empty: the price holds
// at every hour.
type Peak string

const (
	// PeakWeekdays is 01:00 to 04:00 and 06:00 to 10:00 UTC on every
	// weekday, holidays too: what
	// Command Code (https://commandcode.ai/docs/resources/pricing-limits)
	// and OpenCode Go (https://opencode.ai/docs/go) bill DeepSeek's models
	// at, neither of which names a holiday
	PeakWeekdays Peak = "weekdays"
	// PeakOllama is Ollama Cloud's: 12:00 to 18:00 UTC, Monday to Friday
	// (https://ollama.com/pricing: off-peak outside 12:00 and 18:00 UTC on
	// weekdays, and all of Saturday and Sunday), where its
	// off-peak rows are half its peak ones and models.dev lists the
	// off-peak row as the price. It names no holiday.
	PeakOllama Peak = "ollama"
)

// PeakFactor is what a call in the peak hours costs, times the off-peak
// list price: DeepSeek's off-peak rates are half its peak rates, and so
// are Ollama Cloud's.
const PeakFactor = 2

// At is whether a call made at t is in the peak hours. A zero time, a
// call whose time isn't known, is not.
func (k Peak) At(t time.Time) bool {
	if k == "" || t.IsZero() {
		return false
	}
	u := t.UTC()
	if wd := u.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return false
	}
	h := u.Hour()
	if k == PeakOllama {
		return h >= 12 && h < 18
	}
	return h >= 1 && h < 4 || h >= 6 && h < 10
}

// In is the price a call made at t is billed at: PeakFactor times this one
// in its peak hours, else this one. The price it gives holds at every hour.
func (p Price) In(t time.Time) Price {
	if !p.Peak.At(t) {
		p.Peak = ""
		return p
	}
	p = p.Times(PeakFactor)
	p.Peak = ""
	return p
}
