package structuredpath

import "testing"

func TestCanonicalizeAliasPaths(t *testing.T) {
	tests := map[string]string{
		"steps.0.spec.timeout":                   "/steps/0/spec/timeout",
		"steps[3].spec.timeout":                  "/steps/3/spec/timeout",
		"a[0][1]":                                "/a/0/1",
		"a[\"b.c\"]":                             "/a/b.c",
		"a[\"b/c\"]":                             "/a/b~1c",
		"/a/b~1c":                                "/a/b~1c",
		"steps.apply-runtime-ready.spec.timeout": "/steps/apply-runtime-ready/spec/timeout",
	}
	for in, want := range tests {
		got, err := Canonicalize(in)
		if err != nil {
			t.Fatalf("Canonicalize(%q): %v", in, err)
		}
		if got != want {
			t.Fatalf("Canonicalize(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseRejectsBrokenAlias(t *testing.T) {
	for _, in := range []string{"steps..timeout", "steps[", `steps["unterminated]`} {
		if _, err := Parse(in); err == nil {
			t.Fatalf("expected Parse(%q) to fail", in)
		}
	}
}
