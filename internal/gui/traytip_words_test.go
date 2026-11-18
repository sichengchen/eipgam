package gui

import (
	"testing"
)

// parsePageWords reads every entry of a block, two to a line too, with
// its escapes, and leaves out what isn't an entry.
func TestParsePageWords(t *testing.T) {
	src := "const I18N = {\n  zh: {\n    // a comment: \"not\": \"this\"\n    \"Repository\": \"仓库\", \"Branch\": \"分支\",\n    \"say \\\"{n}\\\"\": \"说「{n}」\",\n  },\n  \"zh-TW\": {\n    \"Credits\": \"積分\",\n  },\n};\nconst NOTICES = [\n  \"Credits\": \"no\",\n];\n"
	got := parsePageWords(src)
	if got["zh"]["Repository"] != "仓库" || got["zh"]["Branch"] != "分支" || got["zh"][`say "{n}"`] != "说「{n}」" || got["zh-TW"]["Credits"] != "積分" {
		t.Errorf("parsed %v", got)
	}
	if _, ok := got["zh"]["not"]; ok || len(got) != 2 {
		t.Errorf("parsed a comment or past the table: %v", got)
	}
}
