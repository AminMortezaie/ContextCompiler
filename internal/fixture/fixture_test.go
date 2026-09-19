package fixture

import (
	"testing"
)

func TestDay0HasGoldenRelevantIDs(t *testing.T) {
	store, task := Day0()
	if len(store.All()) < 10 {
		t.Fatalf("expected >=10 entities, got %d", len(store.All()))
	}
	if task.Question == "" {
		t.Fatal("empty question")
	}
	for _, id := range task.RelevantIDs {
		if _, ok := store.Get(id); !ok {
			t.Errorf("relevant ID %q missing from store", id)
		}
	}
	for _, id := range []string{"doc-handbook", "evt-party", "tkt-099", "proj-y"} {
		if _, ok := store.Get(id); !ok {
			t.Errorf("noise entity %q missing", id)
		}
	}
}

func TestScale100KApprox(t *testing.T) {
	st, task := Scale(TargetTokens100K, DefaultSeed)
	est := EstimateStoreTokens(st)
	// Allow ±5% around target.
	lo := TargetTokens100K * 95 / 100
	hi := TargetTokens100K * 105 / 100
	if est < lo || est > hi+5000 { // slight overshoot OK from last entity
		t.Fatalf("est_tokens=%d want ~%d (lo=%d)", est, TargetTokens100K, lo)
	}
	for _, id := range task.RelevantIDs {
		if _, ok := st.Get(id); !ok {
			t.Errorf("golden ID %q missing from scaled store", id)
		}
	}
	if len(st.All()) < 100 {
		t.Fatalf("expected many noise entities, got %d", len(st.All()))
	}
	t.Logf("Scale100K: entities=%d est_tokens=%d", len(st.All()), est)
}

func TestScaleDeterministic(t *testing.T) {
	a, _ := Scale(5_000, 42)
	b, _ := Scale(5_000, 42)
	if len(a.All()) != len(b.All()) {
		t.Fatalf("non-deterministic length %d vs %d", len(a.All()), len(b.All()))
	}
	for i := range a.All() {
		if a.All()[i].ID != b.All()[i].ID {
			t.Fatalf("ID mismatch at %d: %s vs %s", i, a.All()[i].ID, b.All()[i].ID)
		}
	}
}

func TestScale500KApprox(t *testing.T) {
	st, task := Scale(TargetTokens500K, DefaultSeed)
	est := EstimateStoreTokens(st)
	lo := TargetTokens500K * 95 / 100
	hi := TargetTokens500K * 105 / 100
	if est < lo || est > hi+10000 {
		t.Fatalf("est_tokens=%d want ~%d (lo=%d)", est, TargetTokens500K, lo)
	}
	for _, id := range task.RelevantIDs {
		if _, ok := st.Get(id); !ok {
			t.Errorf("golden ID %q missing from scaled store", id)
		}
	}
	if len(st.All()) < 500 {
		t.Fatalf("expected many noise entities, got %d", len(st.All()))
	}
	t.Logf("Scale500K: entities=%d est_tokens=%d", len(st.All()), est)
}

func TestScale1MApprox(t *testing.T) {
	st, task := Scale(TargetTokens1M, DefaultSeed)
	est := EstimateStoreTokens(st)
	lo := TargetTokens1M * 95 / 100
	if est < lo || est > TargetTokens1M+20000 {
		t.Fatalf("est_tokens=%d want ~%d (lo=%d)", est, TargetTokens1M, lo)
	}
	for _, id := range task.RelevantIDs {
		if _, ok := st.Get(id); !ok {
			t.Errorf("golden ID %q missing from scaled store", id)
		}
	}
	if len(st.All()) < 1000 {
		t.Fatalf("expected lots of noise entities, got %d", len(st.All()))
	}
	t.Logf("Scale1M: entities=%d est_tokens=%d", len(st.All()), est)
}

func TestScale5MApprox(t *testing.T) {
	st, task := Scale(TargetTokens5M, DefaultSeed)
	est := EstimateStoreTokens(st)
	lo := TargetTokens5M * 95 / 100
	if est < lo || est > TargetTokens5M+50000 {
		t.Fatalf("est_tokens=%d want ~%d (lo=%d)", est, TargetTokens5M, lo)
	}
	for _, id := range task.RelevantIDs {
		if _, ok := st.Get(id); !ok {
			t.Errorf("golden ID %q missing from scaled store", id)
		}
	}
	if len(st.All()) < 5000 {
		t.Fatalf("expected lots of noise entities, got %d", len(st.All()))
	}
	t.Logf("Scale5M: entities=%d est_tokens=%d", len(st.All()), est)
}
