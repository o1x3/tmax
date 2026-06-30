package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadClaudeDedup verifies the per-turn deduplication: Claude writes one
// JSONL line per content block of an assistant turn, each repeating the same
// cumulative usage. We must count each turn's usage exactly once, count real
// user prompts, and skip tool_result lines.
func TestLoadClaudeDedup(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".claude", "projects", "p")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ts := "2026-06-20T10:00:00.000Z"
	asst := func(id string, in, out, cr, cw int) string {
		return fmt.Sprintf(`{"type":"assistant","timestamp":%q,"requestId":"req_%s","message":{"id":"msg_%s","role":"assistant","model":"claude-opus-4-8","usage":{"input_tokens":%d,"output_tokens":%d,"cache_read_input_tokens":%d,"cache_creation_input_tokens":%d}}}`,
			ts, id, id, in, out, cr, cw)
	}
	lines := []string{
		asst("1", 100, 50, 200, 10), // turn 1 — block a
		asst("1", 100, 50, 200, 10), // turn 1 — block b (duplicate)
		asst("1", 100, 50, 200, 10), // turn 1 — block c (duplicate)
		`{"type":"user","timestamp":"` + ts + `","message":{"role":"user","content":"hello"}}`,
		`{"type":"user","timestamp":"` + ts + `","message":{"role":"user","content":[{"type":"tool_result","content":"ok"}]}}`,
		asst("2", 5, 5, 0, 0), // turn 2
	}
	if err := os.WriteFile(filepath.Join(dir, "s.jsonl"), []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	a := loadClaude()

	if a.InputTokens != 105 {
		t.Errorf("InputTokens = %d, want 105 (turn1 100 + turn2 5)", a.InputTokens)
	}
	if a.OutputTokens != 55 {
		t.Errorf("OutputTokens = %d, want 55", a.OutputTokens)
	}
	if a.CacheReadTokens != 200 {
		t.Errorf("CacheReadTokens = %d, want 200", a.CacheReadTokens)
	}
	if a.CacheWriteTokens != 10 {
		t.Errorf("CacheWriteTokens = %d, want 10", a.CacheWriteTokens)
	}
	if a.TotalTokens() != 370 {
		t.Errorf("TotalTokens = %d, want 370 (360 + 10), not the 3x-inflated total", a.TotalTokens())
	}
	if a.Messages != 3 {
		t.Errorf("Messages = %d, want 3 (2 assistant turns + 1 real user; tool_result skipped)", a.Messages)
	}
	if a.Sessions != 1 {
		t.Errorf("Sessions = %d, want 1", a.Sessions)
	}
	models := a.TopModels()
	if len(models) != 1 || models[0].ID != "claude-opus-4-8" || models[0].Tokens != 370 {
		t.Errorf("TopModels = %+v, want single opus-4-8 with 370 tokens", models)
	}
}

// TestScanLinesHugeLine ensures a single oversized line doesn't truncate the
// rest of the file (the old bufio.Scanner cap bug).
func TestScanLinesHugeLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.jsonl")
	huge := strings.Repeat("x", 70*1024*1024) // 70MB single line
	content := "first\n" + huge + "\nlast\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	scanLines(path, func(b []byte) {
		if len(b) < 100 { // ignore the huge line's content
			got = append(got, string(b))
		}
	})
	if len(got) != 2 || got[0] != "first" || got[1] != "last" {
		t.Errorf("scanLines dropped lines around a huge line: got %v", got)
	}
}
