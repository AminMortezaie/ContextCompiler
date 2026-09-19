// Package tokens provides a simple labeled token estimator (chars/4).
package tokens

// CharsPerToken is the Day-0 heuristic: ~4 characters per token.
// Labeled clearly so Day-1 can swap in a real tokenizer.
const CharsPerToken = 4

// Estimate returns an approximate token count for s using chars/4.
func Estimate(s string) int {
	n := len(s)
	if n == 0 {
		return 0
	}
	return (n + CharsPerToken - 1) / CharsPerToken
}

// CostPer1KTokensUSD is a constant mock pricing used for Day-0 cost estimates.
const CostPer1KTokensUSD = 0.002 // $0.002 per 1K tokens (mock)

// EstimateCostUSD estimates dollar cost from total token count.
func EstimateCostUSD(totalTokens int) float64 {
	return float64(totalTokens) / 1000.0 * CostPer1KTokensUSD
}
