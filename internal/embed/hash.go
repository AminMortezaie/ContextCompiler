package embed

import (
	"context"
	"hash/fnv"
	"math"
	"strings"
	"unicode"
)

// HashProvider is a deterministic local embedding: tokenize → multi-hash
// bag-of-words into Dim dims → L2-normalize. No network, fixed Dim=384.
//
// Tuned for short discriminative signals (titles + lead sentences). Callers
// should prefer EmbedDocument(title, body) which truncates long bodies so
// padded noise cannot dominate via hash collisions.
type HashProvider struct{}

func NewHash() *HashProvider { return &HashProvider{} }

func (h *HashProvider) Name() string { return "hash-bow-384" }
func (h *HashProvider) Dim() int     { return Dim }

func (h *HashProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = hashEmbed(t, Dim, 1.0)
	}
	return out, nil
}

// EmbedDocument embeds title (3x weight) + truncated body for indexing/querying.
func EmbedDocument(ctx context.Context, p Provider, title, body string) ([]float32, error) {
	const maxBody = 280 // chars — keeps noise padding from drowning lexical signal
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	// If provider is HashProvider, apply title boost directly.
	if hp, ok := p.(*HashProvider); ok {
		_ = hp
		return hashEmbedWeighted(title, body, Dim), nil
	}
	return EmbedOne(ctx, p, title+"\n"+body)
}

func hashEmbedWeighted(title, body string, dim int) []float32 {
	vec := make([]float32, dim)
	addTokens(vec, tokenizeEmbed(title), 3.0)
	addTokens(vec, tokenizeEmbed(body), 1.0)
	return l2normalize(vec)
}

func hashEmbed(text string, dim int, weight float32) []float32 {
	vec := make([]float32, dim)
	addTokens(vec, tokenizeEmbed(text), weight)
	return l2normalize(vec)
}

func addTokens(vec []float32, toks []string, weight float32) {
	dim := len(vec)
	for _, tok := range toks {
		tok = lightStem(tok)
		if len(tok) < 2 {
			continue
		}
		// Three independent hashes reduce collision clumping.
		for salt := 0; salt < 3; salt++ {
			h := fnv.New32a()
			_, _ = h.Write([]byte{byte(salt)})
			_, _ = h.Write([]byte(tok))
			idx := int(h.Sum32() % uint32(dim))
			vec[idx] += weight
		}
	}
}

func lightStem(s string) string {
	// Tiny stemmer so "delayed"/"delay" and "decisions"/"decision" align.
	for _, suf := range []string{"ingly", "edly", "ing", "ed", "ly", "es", "s"} {
		if len(s) > len(suf)+3 && strings.HasSuffix(s, suf) {
			return s[:len(s)-len(suf)]
		}
	}
	return s
}

func tokenizeEmbed(s string) []string {
	s = strings.ToLower(s)
	var b strings.Builder
	var out []string
	flush := func() {
		if b.Len() >= 2 {
			out = append(out, b.String())
		}
		b.Reset()
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			b.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func l2normalize(v []float32) []float32 {
	var sum float64
	for _, x := range v {
		sum += float64(x) * float64(x)
	}
	if sum == 0 {
		return v
	}
	inv := float32(1.0 / math.Sqrt(sum))
	for i := range v {
		v[i] *= inv
	}
	return v
}

// Cosine returns cosine similarity of two equal-length vectors.
func Cosine(a, b []float32) float64 {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	var dot, na, nb float64
	for i := 0; i < n; i++ {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
