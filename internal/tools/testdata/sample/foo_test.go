package sample

import "testing"

func TestFooDoSomething(t *testing.T) {
	f := &Foo{ID: 7}

	if f.DoSomething() == "" {
		t.Fatal("expected DoSomething to return non-empty string")
	}
}
