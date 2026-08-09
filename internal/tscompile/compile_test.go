package tscompile

import (
	"context"
	"strings"
	"testing"
)

func TestHashSourceStable(t *testing.T) {
	a := HashSource("TYPESCRIPT", "export default function() {}")
	b := HashSource("TYPESCRIPT", "export default function() {}")
	c := HashSource("TYPESCRIPT", "export default function() {}\n")
	if a != b {
		t.Fatal("unstable")
	}
	if a == c {
		t.Fatal("should differ")
	}
}

func TestMockCompile(t *testing.T) {
	c := New(Options{Mock: true})
	b, err := c.Compile(context.Background(), "TYPESCRIPT", "export default function handler() {}")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "export default") {
		t.Fatalf("bad js: %s", b)
	}
}

func TestEsbuildCompile(t *testing.T) {
	c := New(Options{})
	src := `
export default function handler({ ctx, body }: { ctx?: unknown; body?: unknown }) {
  const n: number = 1;
  return { ok: true, n, body };
}
`
	b, err := c.Compile(context.Background(), "TS", src)
	if err != nil {
		t.Fatal(err)
	}
	out := string(b)
	if !strings.Contains(out, "export") || !strings.Contains(out, "default") {
		t.Fatalf("expected default export, got: %s", out)
	}
	if strings.Contains(out, ": number") {
		t.Fatalf("type annotations should be stripped: %s", out)
	}
}

func TestNormalize(t *testing.T) {
	if NormalizeSourceType("ts") != SourceTypeScript {
		t.Fatal("ts")
	}
	if NormalizeSourceType("") != SourceTypeScript {
		t.Fatal("default TYPESCRIPT")
	}
	if !NeedsCompile("TYPESCRIPT") {
		t.Fatal("TYPESCRIPT should compile")
	}
}
