package tscompile

import "testing"

func TestArtifactKeyIncludesGroup(t *testing.T) {
	got := ArtifactKey("sol_1", "doThing", "abc")
	want := "sol_1/doThing/abc.js"
	if got != want {
		t.Fatalf("ArtifactKey=%q want %q", got, want)
	}
	if SourceKey("", "doThing", "abc") != "_/doThing/abc.ts" {
		t.Fatalf("empty group should use _")
	}
}
