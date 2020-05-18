package crossplane

import (
	"fmt"
	"strings"
	"unicode"
)

type included struct {
	directive Directive
	err       error
}

func contains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}

func validFlag(s string) bool {
	l := strings.ToLower(s)
	return l == "on" || l == "off"
}

// prepareIfArgs removes parentheses from an `if` directive's arguments.
func prepareIfArgs(d Directive) Directive {
	e := len(d.Args) - 1
	if len(d.Args) > 0 && strings.HasPrefix(d.Args[0], "(") && strings.HasSuffix(d.Args[e], ")") {
		d.Args[0] = strings.TrimLeftFunc(strings.TrimPrefix(d.Args[0], "("), unicode.IsSpace)
		d.Args[e] = strings.TrimRightFunc(strings.TrimSuffix(d.Args[e], ")"), unicode.IsSpace)
		if len(d.Args[0]) == 0 {
			d.Args = d.Args[1:]
		}
		if len(d.Args[e]) == 0 {
			d.Args = d.Args[:e]
		}
	}
	return d
}

// combineConfigs combines config files into one by using include directives.
func combineConfigs(old Payload) (*Payload, error) {
	if len(old.Config) < 1 {
		return &old, nil
	}

	status := old.Status
	if status == "" {
		status = "ok"
	}

	errors := old.Errors
	if errors == nil {
		errors = []PayloadError{}
	}

	combined := Config{
		File:   old.Config[0].File,
		Status: "ok",
		Errors: []ConfigError{},
		Parsed: []Directive{},
	}

	for _, config := range old.Config {
		combined.Errors = append(combined.Errors, config.Errors...)
		if config.Status == "failed" {
			combined.Status = "failed"
		}
	}

	for incl := range performIncludes(old, combined.File, old.Config[0].Parsed) {
		if incl.err != nil {
			return nil, incl.err
		}
		combined.Parsed = append(combined.Parsed, incl.directive)
	}

	return &Payload{
		Status: status,
		Errors: errors,
		Config: []Config{combined},
	}, nil
}

func performIncludes(old Payload, fromfile string, block []Directive) chan included {
	c := make(chan included)
	go func() {
		defer close(c)

		for _, dir := range block {
			if dir.IsBlock() {
				block := []Directive{}
				for incl := range performIncludes(old, fromfile, *dir.Block) {
					if incl.err != nil {
						c <- incl
						return
					}
					block = append(block, incl.directive)
				}
				dir.Block = &block
			}

			if !dir.IsInclude() {
				c <- included{directive: dir}
				continue
			}

			for _, idx := range *dir.Includes {
				if idx >= len(old.Config) {
					c <- included{
						err: ParseError{
							what: fmt.Sprintf("include config with index: %d", idx),
							file: &fromfile,
							line: &dir.Line,
						},
					}
					return
				}
				for incl := range performIncludes(old, old.Config[idx].File, old.Config[idx].Parsed) {
					c <- incl
				}
			}
		}

	}()
	return c
}
