package jscache

import "testing"

func TestLRUEvict(t *testing.T) {
	c := New(2)
	c.Put("a", Entry{Name: "a", Etag: "1", JS: "aa"})
	c.Put("b", Entry{Name: "b", Etag: "1", JS: "bb"})
	c.Put("c", Entry{Name: "c", Etag: "1", JS: "cc"})
	if _, ok := c.Get("a"); ok {
		t.Fatal("a should be evicted")
	}
	if e, ok := c.Get("b"); !ok || e.JS != "bb" {
		t.Fatalf("b: %#v ok=%v", e, ok)
	}
	c.Put("d", Entry{Name: "d", Etag: "1", JS: "dd"})
	if _, ok := c.Get("c"); ok {
		t.Fatal("c should be evicted after touching b")
	}
}
