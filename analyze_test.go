package crossplane

import (
	"strings"
	"testing"
)

func TestAnalyze(t *testing.T) {
	fname := "/path/to/nginx.conf"
	ctx := blockCtx{"events"}

	// Checks that the `state` directive should only be in certain contexts.
	t.Run("state-directive", func(t *testing.T) {
		stmt := Directive{
			Directive: "state",
			Args:      []string{"/path/to/state/file.conf"},
			Line:      5, // this is arbitrary
		}

		// the state directive should not cause errors if it"s in these contexts
		goodCtxs := []blockCtx{
			blockCtx{"http", "upstream"},
			blockCtx{"stream", "upstream"},
			blockCtx{"some_third_party_context"},
		}
		for _, ctx := range goodCtxs {
			if err := analyze(fname, stmt, ";", ctx, &ParseOptions{}); err != nil {
				t.Fatalf("expected err to be nil: %v", err)
			}
		}
		goodMap := map[string]bool{}
		for _, c := range goodCtxs {
			goodMap[c.key()] = true
		}

		for key, _ := range contexts {
			// the state directive should only be in the "good" contexts
			if _, ok := goodMap[key]; !ok {
				ctx := blockCtx(strings.Split(key, ">"))
				if err := analyze(fname, stmt, ";", ctx, &ParseOptions{}); err == nil {
					t.Fatalf("expected error to not be nil: %v", err)
				} else if e, ok := err.(ParseError); !ok {
					t.Fatalf("error was not a ParseError: %v", err)
				} else if !strings.HasSuffix(e.what, `directive is not allowed here`) {
					t.Fatalf("unexpected error message: %q", e.what)
				}
			}
		}
	})

	// Check which arguments are valid for flag directives.
	t.Run("flag-args", func(t *testing.T) {
		stmt := Directive{
			Directive: "accept_mutex",
			Line:      2, // this is arbitrary
		}

		goodArgs := [][]string{[]string{"on"}, []string{"off"}, []string{"On"}, []string{"Off"}, []string{"ON"}, []string{"OFF"}}
		for _, args := range goodArgs {
			stmt.Args = args
			if err := analyze(fname, stmt, ";", ctx, &ParseOptions{}); err != nil {
				t.Fatalf("expected err to be nil: %v", err)
			}
		}

		badArgs := [][]string{[]string{"1"}, []string{"0"}, []string{"true"}, []string{"okay"}, []string{""}}
		for _, args := range badArgs {
			stmt.Args = args
			if err := analyze(fname, stmt, ";", ctx, &ParseOptions{}); err == nil {
				t.Fatalf("expected error to not be nil: %v", err)
			} else if e, ok := err.(ParseError); !ok {
				t.Fatalf("error was not a ParseError: %v", err)
			} else if !strings.HasSuffix(e.what, `it must be "on" or "off"`) {
				t.Fatalf("unexpected error message: %q", e.what)
			}
		}
	})
}
