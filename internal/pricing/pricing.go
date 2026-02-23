package pricing

import "strings"

// per million tokens
type ModelPrice struct {
	Input  float64
	Output float64
}

var prices = map[string]ModelPrice{
	"claude-3-5-sonnet":    {3.0, 15.0},
	"claude-sonnet-4":      {3.0, 15.0},
	"claude-3-5-haiku":     {0.8, 4.0},
	"claude-3-haiku":       {0.25, 1.25},
	"claude-3-opus":        {15.0, 75.0},
	"claude-opus-4":        {15.0, 75.0},
	"gpt-4o":               {2.5, 10.0},
	"gpt-4o-mini":          {0.15, 0.6},
	"gpt-4-turbo":          {10.0, 30.0},
	"gpt-4.1":              {2.0, 8.0},
	"gpt-4.1-mini":         {0.4, 1.6},
	"gpt-4.1-nano":         {0.1, 0.4},
	"o1":                   {15.0, 60.0},
	"o1-mini":              {1.1, 4.4},
	"o1-pro":               {150.0, 600.0},
	"o3":                   {2.0, 8.0},
	"o3-mini":              {1.1, 4.4},
	"o4-mini":              {1.1, 4.4},
	"deepseek-chat":        {0.27, 1.10},
	"deepseek-coder":       {0.14, 0.28},
	"deepseek-reasoner":    {0.55, 2.19},
}

func Lookup(model string) (ModelPrice, bool) {
	m := strings.ToLower(model)
	// exact match first
	if p, ok := prices[m]; ok {
		return p, true
	}
	// prefix match
	for k, p := range prices {
		if strings.HasPrefix(m, k) {
			return p, true
		}
	}
	return ModelPrice{}, false
}

func Cost(model string, inputTokens, outputTokens int) float64 {
	p, ok := Lookup(model)
	if !ok {
		return 0
	}
	return float64(inputTokens)/1e6*p.Input + float64(outputTokens)/1e6*p.Output
}
