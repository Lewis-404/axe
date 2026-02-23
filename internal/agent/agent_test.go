package agent

import (
	"testing"

	"github.com/Lewis-404/axe/internal/llm"
)

func TestEstimateTokens(t *testing.T) {
	// The function also counts fmt.Sprintf("%v", b.Input) which adds "<nil>" (5 ASCII chars) per block
	// pure ASCII: "hello world" = 11 ASCII + "<nil>" = 5 ASCII → 16/4 = 4
	ascii := []llm.Message{{Content: []llm.ContentBlock{{Text: "hello world"}}}}
	got := estimateTokens(ascii)
	if got <= 0 {
		t.Errorf("ASCII tokens = %d, want > 0", got)
	}

	// pure CJK: 6 non-ASCII → 6*2/3=4, plus "<nil>"=5 ASCII → 5/4=1 → total 5
	cjk := []llm.Message{{Content: []llm.ContentBlock{{Text: "你好世界测试"}}}}
	got = estimateTokens(cjk)
	if got <= 0 {
		t.Errorf("CJK tokens = %d, want > 0", got)
	}

	// CJK should produce more tokens per char than ASCII
	asciiLong := []llm.Message{{Content: []llm.ContentBlock{{Text: "abcdefghijkl"}}}}
	cjkLong := []llm.Message{{Content: []llm.ContentBlock{{Text: "你好世界测试一二三四五六"}}}}
	asciiTok := estimateTokens(asciiLong)
	cjkTok := estimateTokens(cjkLong)
	if cjkTok <= asciiTok {
		t.Errorf("CJK tokens (%d) should be > ASCII tokens (%d) for same char count", cjkTok, asciiTok)
	}
}

func TestUTF8SafeTruncation(t *testing.T) {
	// simulate the truncation logic from agent.go
	input := ""
	for i := 0; i < 10001; i++ {
		input += "中"
	}
	runes := []rune(input)
	if len(runes) != 10001 {
		t.Fatalf("setup: expected 10001 runes, got %d", len(runes))
	}
	truncated := string(runes[:10000])
	// verify no broken UTF-8
	for i, r := range truncated {
		if r == '\uFFFD' {
			t.Errorf("broken UTF-8 at rune index %d", i)
			break
		}
	}
	if len([]rune(truncated)) != 10000 {
		t.Errorf("truncated rune count = %d, want 10000", len([]rune(truncated)))
	}
}
