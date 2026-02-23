package pricing

import "testing"

func TestCostKnownModels(t *testing.T) {
	cases := []struct {
		model    string
		in, out  int
		wantZero bool
	}{
		{"claude-sonnet-4", 1000, 1000, false},
		{"claude-opus-4", 1000, 1000, false},
		{"gpt-4o", 1000, 1000, false},
		{"gpt-4.1", 1000, 1000, false},
		{"gpt-4.1-mini", 1000, 1000, false},
		{"o3", 1000, 1000, false},
		{"o4-mini", 1000, 1000, false},
		{"deepseek-reasoner", 1000, 1000, false},
		{"unknown-model-xyz", 1000, 1000, true},
	}
	for _, c := range cases {
		got := Cost(c.model, c.in, c.out)
		if c.wantZero && got != 0 {
			t.Errorf("Cost(%q) = %f, want 0", c.model, got)
		}
		if !c.wantZero && got <= 0 {
			t.Errorf("Cost(%q) = %f, want > 0", c.model, got)
		}
	}
}

func TestCostCalculation(t *testing.T) {
	// claude-sonnet-4: $3/M in, $15/M out
	cost := Cost("claude-sonnet-4", 1_000_000, 1_000_000)
	expected := 3.0 + 15.0
	if cost < expected-0.01 || cost > expected+0.01 {
		t.Errorf("Cost = %f, want ~%f", cost, expected)
	}
}
