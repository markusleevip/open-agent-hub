package services

import "testing"

func TestTokenize_CJKBigramAndLatin(t *testing.T) {
	got := Tokenize("数据库")
	want := []string{"数据", "据库"}
	if len(got) != len(want) {
		t.Fatalf("'数据库' should produce %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: expected %q, got %q", i, want[i], got[i])
		}
	}

	if g := Tokenize("猫"); len(g) != 1 || g[0] != "猫" {
		t.Fatalf("single character should yield [猫], got %v", g)
	}
	if g := Tokenize("Hello World"); len(g) != 2 || g[0] != "hello" || g[1] != "world" {
		t.Fatalf("Latin tokenization wrong: %v", g)
	}
	if g := Tokenize("用 Go 写"); len(g) != 3 || g[0] != "用" || g[1] != "go" || g[2] != "写" {
		t.Fatalf("mixed CJK/Latin tokenization wrong: %v", g)
	}
}

// Core bug before fix: Chinese similarity was always 0
func TestRelevance_ChineseNoLongerZero(t *testing.T) {
	q := "数据库连接池配置"
	doc := "本项目的数据库连接池配置在 config.go 中，最大连接数为 20"
	if s := Relevance(q, doc); s <= 0 {
		t.Fatalf("Chinese query should recall Chinese document, relevance=%v", s)
	}
	if s := Relevance("数据库", "数据库连接池"); s < 0.99 {
		t.Fatalf("fully covered query should score ~1.0, got %v", s)
	}
	if s := Relevance("天气预报", "数据库连接池配置"); s > 0.2 {
		t.Fatalf("unrelated content should score low, got %v", s)
	}
}

func TestRelevance_EnglishStillWorks(t *testing.T) {
	if s := Relevance("database pool", "the database connection pool is configured here"); s < 0.99 {
		t.Fatalf("fully covered English query should score ~1.0, got %v", s)
	}
	if s := Relevance("kubernetes ingress", "database connection pool settings"); s != 0 {
		t.Fatalf("completely unrelated English should score 0, got %v", s)
	}
}

func TestSimilarity_SymmetricForDedup(t *testing.T) {
	if s := Similarity("缓存使用 Redis 实现", "缓存使用 Redis 实现"); s < 0.99 {
		t.Fatalf("identical strings should score ~1.0, got %v", s)
	}
	a, b := "数据库连接池", "数据库"
	if Similarity(a, b) != Similarity(b, a) {
		t.Fatal("Similarity should be symmetric")
	}
	if s := Similarity("缓存", "缓存使用 Redis 集群并配置持久化与主从复制策略"); s >= 0.92 {
		t.Fatalf("short content should not be flagged as duplicate of existing memory, got %v", s)
	}
	if Similarity("", "abc") != 0 || Relevance("", "abc") != 0 {
		t.Fatal("empty input should score 0")
	}
}
