package gateway

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// nullAsUnicode is a pattern fix for toolPatterns' tests: the ECMAScript
// null escape \0 as the Unicode escape \u0000.
func nullAsUnicode(pattern string) string { return strings.ReplaceAll(pattern, `\0`, `\u0000`) }

func TestToolPatternsOnlySchemas(t *testing.T) {
	input := []byte(`{"messages":[{"role":"user","content":"\\0"}],"tools":[{"name":"Artifact","input_schema":{"type":"object","properties":{"content":{"type":"string","pattern":"^[^\\0]*$","default":{"pattern":"\\0"}}},"const":{"pattern":"\\0"}}},{"type":"function","function":{"name":"other","parameters":{"anyOf":[{"pattern":"\\0"}]}}}]}`)
	out := toolPatterns(input, nullAsUnicode)
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	tools := got["tools"].([]any)
	schema := tools[0].(map[string]any)["input_schema"].(map[string]any)
	field := schema["properties"].(map[string]any)["content"].(map[string]any)
	if field["pattern"] != `^[^\u0000]*$` {
		t.Fatal("Artifact pattern not normalized")
	}
	if field["default"].(map[string]any)["pattern"] != `\0` || schema["const"].(map[string]any)["pattern"] != `\0` {
		t.Fatal("schema data changed")
	}
	if got["messages"].([]any)[0].(map[string]any)["content"] != `\0` {
		t.Fatal("user message changed")
	}
	other := tools[1].(map[string]any)["function"].(map[string]any)["parameters"].(map[string]any)
	if other["anyOf"].([]any)[0].(map[string]any)["pattern"] != `\u0000` {
		t.Fatal("Chat function schema not normalized")
	}
	unchanged := []byte(` {"tools":[{"input_schema":{"pattern":"^https?://"}}]} `)
	if !bytes.Equal(toolPatterns(unchanged, nullAsUnicode), unchanged) {
		t.Fatal("unchanged request was re-encoded")
	}
}

func TestToolPatternsByteIdentity(t *testing.T) {

	// 1. A body with nothing to fix passes through byte for byte.
	nothingToFix := []byte(`{
		"z_order": 1,
		"a_order": 2,
		"html_description": "User description with <special> & characters",
		"tools": [
			{"name": "test", "description": "<escapes> & unescaped", "input_schema": {"type": "object", "properties": {"url": {"pattern": "^https?://.*$"}}}}
		]
	}`)
	if got := toolPatterns(nothingToFix, nullAsUnicode); !bytes.Equal(got, nothingToFix) {
		t.Fatalf("body with nothing to fix did not pass through byte for byte:\ngot : %s\nwant: %s", got, nothingToFix)
	}

	// 2. A body with a pattern to fix differs ONLY at the pattern, preserving
	// key order, whitespaces, and <, >, & characters in tool descriptions.
	fixedBodyInput := []byte(`{
		"b_key": "val",
		"a_key": "val",
		"tools": [
			{
				"name": "Artifact",
				"description": "Writes files with <tags> & &amp; entities",
				"input_schema": {
					"type": "object",
					"properties": {
						"content": {"type": "string", "pattern": "^[^\\0]*$"}
					}
				}
			}
		],
		"messages": [{"role": "user", "content": "hello <world> & fun"}]
	}`)
	wantFixedBody := []byte(`{
		"b_key": "val",
		"a_key": "val",
		"tools": [
			{
				"name": "Artifact",
				"description": "Writes files with <tags> & &amp; entities",
				"input_schema": {
					"type": "object",
					"properties": {
						"content": {"type": "string", "pattern": "^[^\\u0000]*$"}
					}
				}
			}
		],
		"messages": [{"role": "user", "content": "hello <world> & fun"}]
	}`)

	gotFixed := toolPatterns(fixedBodyInput, nullAsUnicode)
	if !bytes.Equal(gotFixed, wantFixedBody) {
		t.Fatalf("fixed body differed by more than the pattern:\ngot : %s\nwant: %s", gotFixed, wantFixedBody)
	}
}

func TestToolPatternsResponsesAndInvalidBodies(t *testing.T) {
	input := []byte(`{"id":9007199254740993,"tools":[{"type":"function","name":"Artifact","parameters":{"$defs":{"content":{"pattern":"^[^\\0]*$"}},"properties":{"value":{"items":{"pattern":"\\0"}}}}}]}`)
	out := toolPatterns(input, nullAsUnicode)
	if bytes.Contains(out, []byte(`\\0`)) || !bytes.Contains(out, []byte(`9007199254740993`)) || !bytes.Contains(out, []byte(`\\u0000`)) {
		t.Fatalf("flattened schema or number not preserved: %s", out)
	}
	for _, in := range [][]byte{[]byte(`{"tools":`), []byte(`null`), []byte(`{"tools":[null]}`)} {
		if !bytes.Equal(toolPatterns(in, nullAsUnicode), in) {
			t.Fatal("invalid or unchanged request modified")
		}
	}
}

func TestToolPatternsSpecialPropertyNames(t *testing.T) {
	// Test special property names like a\b, :x, @prefix, |pipe
	input := []byte(`{
		"tools": [
			{
				"name": "SpecialTools",
				"input_schema": {
					"type": "object",
					"properties": {
						"a\\b": {
							"type": "string",
							"pattern": "^[^\\0]*$"
						},
						":x": {
							"type": "string",
							"pattern": "\\0"
						},
						"@special": {
							"type": "string",
							"pattern": "\\0"
						},
						"a|b": {
							"type": "string",
							"pattern": "\\0"
						}
					}
				}
			}
		]
	}`)

	want := []byte(`{
		"tools": [
			{
				"name": "SpecialTools",
				"input_schema": {
					"type": "object",
					"properties": {
						"a\\b": {
							"type": "string",
							"pattern": "^[^\\u0000]*$"
						},
						":x": {
							"type": "string",
							"pattern": "\\u0000"
						},
						"@special": {
							"type": "string",
							"pattern": "\\u0000"
						},
						"a|b": {
							"type": "string",
							"pattern": "\\u0000"
						}
					}
				}
			}
		]
	}`)

	got := toolPatterns(input, nullAsUnicode)
	if !bytes.Equal(got, want) {
		t.Fatalf("special property names not preserved byte-for-byte:\ngot : %s\nwant: %s", got, want)
	}
}

func TestToolPatternsPreservesCallerSpareCapacity(t *testing.T) {
	src := []byte(`{"tools":[{"name":"Artifact","input_schema":{"pattern":"^[^\\0]*$"}}]}`)

	// Allocate a slice with extra capacity, typical of io.ReadAll or append
	body := make([]byte, len(src), len(src)+4096)
	copy(body, src)

	out := toolPatterns(body, nullAsUnicode)
	if bytes.Equal(body, out) {
		t.Fatal("expected body and out to differ")
	}
	if !bytes.Equal(body, src) {
		t.Fatalf("caller's body slice was mutated in-place:\ngot : %s\nwant: %s", body, src)
	}
	if !bytes.Contains(out, []byte(`\\u0000`)) {
		t.Fatalf("output does not contain normalized pattern: %s", out)
	}
}
