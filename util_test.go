package crossplane

import (
	"encoding/json"
	"testing"
)

func TestPayload(t *testing.T) {
	t.Run("combine", func(t *testing.T) {
		payload := Payload{
			Config: []Config{
				Config{
					File: "example1.conf",
					Parsed: []Directive{
						Directive{
							Directive: "include",
							Args:      []string{"example2.conf"},
							Line:      1,
							Includes:  &[]int{1},
						},
					},
				},
				Config{
					File: "example2.conf",
					Parsed: []Directive{
						Directive{
							Directive: "events",
							Args:      []string{},
							Line:      1,
							Block:     &[]Directive{},
						},
						Directive{
							Directive: "http",
							Args:      []string{},
							Line:      2,
							Block:     &[]Directive{},
						},
					},
				},
			},
		}
		expected := Payload{
			Status: "ok",
			Errors: []PayloadError{},
			Config: []Config{
				Config{
					File:   "example1.conf",
					Status: "ok",
					Errors: []ConfigError{},
					Parsed: []Directive{
						Directive{
							Directive: "events",
							Args:      []string{},
							Line:      1,
							Block:     &[]Directive{},
						},
						Directive{
							Directive: "http",
							Args:      []string{},
							Line:      2,
							Block:     &[]Directive{},
						},
					},
				},
			},
		}
		combined, err := payload.Combined()
		if err != nil {
			t.Fatal(err)
		}
		b1, _ := json.Marshal(expected)
		b2, _ := json.Marshal(*combined)
		if string(b1) != string(b2) {
			t.Fatalf("expected: %s\nbut got: %s", b1, b2)
		}
	})
}
