package services

import (
	"strings"
	"unicode"
)

// textsearch is the single implementation of retrieval scoring, shared by MCP tools and Console REST,
// avoiding divergence where "Chinese works here but breaks there". Lexical retrieval without vectors:
// Latin tokens are split by word, CJK tokens are split by bigram, so both Chinese and English can match.

// isCJK checks whether a rune is a CJK ideograph/kana/hangul character
// (these scripts have no whitespace word boundaries).
func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) ||
		unicode.Is(unicode.Hiragana, r) ||
		unicode.Is(unicode.Katakana, r) ||
		unicode.Is(unicode.Hangul, r)
}

// Tokenize splits text into retrieval tokens, handling both Chinese and English:
//   - Consecutive ASCII/Latin alphanumeric characters are split as "words";
//   - Consecutive CJK characters are split as bigrams (single runes become unigrams),
//     so Chinese gets meaningful overlapping matches without a dictionary.
func Tokenize(s string) []string {
	s = strings.ToLower(s)
	tokens := make([]string, 0, len(s)/2)
	var latin, cjk []rune

	flushLatin := func() {
		if len(latin) > 0 {
			tokens = append(tokens, string(latin))
			latin = latin[:0]
		}
	}
	flushCJK := func() {
		switch {
		case len(cjk) == 1:
			tokens = append(tokens, string(cjk))
		case len(cjk) >= 2:
			for i := 0; i+1 < len(cjk); i++ {
				tokens = append(tokens, string(cjk[i:i+2]))
			}
		}
		cjk = cjk[:0]
	}

	for _, r := range s {
		switch {
		case isCJK(r):
			flushLatin()
			cjk = append(cjk, r)
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			flushCJK()
			latin = append(latin, r)
		default:
			flushLatin()
			flushCJK()
		}
	}
	flushLatin()
	flushCJK()
	return tokens
}

func multiset(tokens []string) map[string]int {
	m := make(map[string]int, len(tokens))
	for _, t := range tokens {
		m[t]++
	}
	return m
}

// Similarity computes symmetric Jaccard similarity (intersection/union), used for memory deduplication.
func Similarity(a, b string) float64 {
	at, bt := Tokenize(a), Tokenize(b)
	if len(at) == 0 || len(bt) == 0 {
		return 0
	}
	setA := multiset(at)
	intersect := 0
	for _, w := range bt {
		if setA[w] > 0 {
			intersect++
			setA[w]--
		}
	}
	union := len(at) + len(bt) - intersect
	if union == 0 {
		return 0
	}
	return float64(intersect) / float64(union)
}

// Relevance computes asymmetric retrieval scoring (fraction of query tokens covered by the doc,
// overlap coefficient).
// Suitable for "short query matching long document" retrieval: score is 1.0 when the query is
// fully covered by the document.
func Relevance(query, doc string) float64 {
	qt, dt := Tokenize(query), Tokenize(doc)
	if len(qt) == 0 || len(dt) == 0 {
		return 0
	}
	setD := multiset(dt)
	intersect := 0
	for _, w := range qt {
		if setD[w] > 0 {
			intersect++
			setD[w]--
		}
	}
	denom := len(qt)
	if len(dt) < denom {
		denom = len(dt)
	}
	return float64(intersect) / float64(denom)
}
