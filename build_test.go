package crossplane

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type buildFixture struct {
	name     string
	options  BuildOptions
	parsed   []Directive
	expected string
}

type buildFilesFixture struct {
	name     string
	options  BuildOptions
	payload  Payload
	expected string
}

type compareFixture struct {
	name    string
	options ParseOptions
}

var buildFixtures = []buildFixture{
	buildFixture{
		name:    "nested-and-multiple-args",
		options: BuildOptions{},
		parsed: []Directive{
			Directive{
				Directive: "events",
				Args:      []string{},
				Block: &[]Directive{
					Directive{
						Directive: "worker_connections",
						Args:      []string{"1024"},
					},
				},
			},
			Directive{
				Directive: "http",
				Args:      []string{},
				Block: &[]Directive{
					Directive{
						Directive: "server",
						Args:      []string{},
						Block: &[]Directive{
							Directive{
								Directive: "listen",
								Args:      []string{"127.0.0.1:8080"},
							},
							Directive{
								Directive: "server_name",
								Args:      []string{"default_server"},
							},
							Directive{
								Directive: "location",
								Args:      []string{"/"},
								Block: &[]Directive{
									Directive{
										Directive: "return",
										Args:      []string{"200", "foo bar baz"},
									},
								},
							},
						},
					},
				},
			},
		},
		expected: strings.Join([]string{
			"events {",
			"    worker_connections 1024;",
			"}",
			"http {",
			"    server {",
			"        listen 127.0.0.1:8080;",
			"        server_name default_server;",
			"        location / {",
			`            return 200 "foo bar baz";`,
			"        }",
			"    }",
			"}",
		}, "\n"),
	},
	buildFixture{
		name:    "with-comments",
		options: BuildOptions{},
		parsed: []Directive{
			Directive{
				Directive: "events",
				Line:      1,
				Args:      []string{},
				Block: &[]Directive{
					Directive{
						Directive: "worker_connections",
						Line:      2,
						Args:      []string{"1024"},
					},
				},
			},
			Directive{
				Directive: "#",
				Line:      4,
				Args:      []string{},
				Comment:   pStr("comment"),
			},
			Directive{
				Directive: "http",
				Line:      5,
				Args:      []string{},
				Block: &[]Directive{
					Directive{
						Directive: "server",
						Line:      6,
						Args:      []string{},
						Block: &[]Directive{
							Directive{
								Directive: "listen",
								Line:      7,
								Args:      []string{"127.0.0.1:8080"},
							},
							Directive{
								Directive: "#",
								Line:      7,
								Args:      []string{},
								Comment:   pStr("listen"),
							},
							Directive{
								Directive: "server_name",
								Line:      8,
								Args:      []string{"default_server"},
							},
							Directive{
								Directive: "location",
								Line:      9,
								Args:      []string{"/"},
								Block: &[]Directive{
									Directive{
										Directive: "#",
										Line:      9,
										Args:      []string{},
										Comment:   pStr("# this is brace"),
									},
									Directive{
										Directive: "#",
										Line:      10,
										Args:      []string{},
										Comment:   pStr(" location /"),
									},
									Directive{
										Directive: "#",
										Line:      11,
										Args:      []string{},
										Comment:   pStr(" is here"),
									},
									Directive{
										Directive: "return",
										Line:      12,
										Args:      []string{"200", "foo bar baz"},
									},
								},
							},
						},
					},
				},
			},
		},
		expected: strings.Join([]string{
			"events {",
			"    worker_connections 1024;",
			"}",
			"#comment",
			"http {",
			"    server {",
			"        listen 127.0.0.1:8080; #listen",
			"        server_name default_server;",
			"        location / { ## this is brace",
			"            # location /",
			"            # is here",
			`            return 200 "foo bar baz";`,
			"        }",
			"    }",
			"}",
		}, "\n"),
	},
	buildFixture{
		name:    "starts-with-comments",
		options: BuildOptions{},
		parsed: []Directive{
			Directive{
				Directive: "#",
				Line:      1,
				Args:      []string{},
				Comment:   pStr(" foo"),
			},
			Directive{
				Directive: "user",
				Line:      5,
				Args:      []string{"root"},
			},
		},
		expected: "# foo\nuser root;",
	},
	buildFixture{
		name:    "with-quoted-unicode",
		options: BuildOptions{},
		parsed: []Directive{
			Directive{
				Directive: "env",
				Line:      1,
				Args:      []string{"русский текст"},
			},
		},
		expected: `env "русский текст";`,
	},
	buildFixture{
		name:    "multiple-comments-on-one-line",
		options: BuildOptions{},
		parsed: []Directive{
			Directive{
				Directive: "#",
				Line:      1,
				Args:      []string{},
				Comment:   pStr("comment1"),
			},
			Directive{
				Directive: "user",
				Line:      2,
				Args:      []string{"root"},
			},
			Directive{
				Directive: "#",
				Line:      2,
				Args:      []string{},
				Comment:   pStr("comment2"),
			},
			Directive{
				Directive: "#",
				Line:      2,
				Args:      []string{},
				Comment:   pStr("comment3"),
			},
		},
		expected: "#comment1\nuser root; #comment2 #comment3",
	},
}

func TestBuild(t *testing.T) {
	for _, fixture := range buildFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Build(&buf, Config{Parsed: fixture.parsed}, &fixture.options); err != nil {
				t.Fatal(err)
			}
			got := buf.String()
			if got != fixture.expected {
				t.Fatalf("expected: %#v\nbut got: %#v", fixture.expected, got)
			}
		})
	}
}

var buildFilesFixtures = []buildFilesFixture{
	buildFilesFixture{
		name:    "with-missing-status-and-errors",
		options: BuildOptions{},
		payload: Payload{
			Config: []Config{
				Config{
					File: "nginx.conf",
					Parsed: []Directive{
						Directive{
							Directive: "user",
							Line:      1,
							Args:      []string{"nginx"},
						},
					},
				},
			},
		},
		expected: "user nginx;\n",
	},
	buildFilesFixture{
		name:    "with-unicode",
		options: BuildOptions{},
		payload: Payload{
			Status: "ok",
			Errors: []PayloadError{},
			Config: []Config{
				Config{
					File:   "nginx.conf",
					Status: "ok",
					Errors: []ConfigError{},
					Parsed: []Directive{
						Directive{
							Directive: "user",
							Line:      1,
							Args:      []string{"測試"},
						},
					},
				},
			},
		},
		expected: "user 測試;\n",
	},
}

func TestBuildFiles(t *testing.T) {
	for _, fixture := range buildFilesFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			tmpdir, err := ioutil.TempDir("", "TestBuildFiles-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpdir)

			if err = BuildFiles(fixture.payload, tmpdir, &fixture.options); err != nil {
				t.Fatal(err)
			}

			content, err := ioutil.ReadFile(filepath.Join(tmpdir, "nginx.conf"))
			if err != nil {
				t.Fatal(err)
			}

			got := string(content)
			if got != fixture.expected {
				t.Fatalf("expected: %#v\nbut got: %#v", fixture.expected, got)
			}
		})
	}
}

var compareFixtures = []compareFixture{
	compareFixture{"simple", ParseOptions{}},
	compareFixture{"messy", ParseOptions{}},
	compareFixture{"with-comments", ParseOptions{ParseComments: true}},
	compareFixture{"empty-value-map", ParseOptions{}},
	compareFixture{"russian-text", ParseOptions{}},
	compareFixture{"quoted-right-brace", ParseOptions{}},
	compareFixture{"directive-with-space", ParseOptions{}},
}

func TestCompareParsedAndBuilt(t *testing.T) {
	for _, fixture := range compareFixtures {
		t.Run(fixture.name, func(t *testing.T) {
			tmpdir, err := ioutil.TempDir("", "TestCompareParsedAndBuilt-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(tmpdir)

			origPayload, err := Parse(filepath.Join("testdata", fixture.name, "nginx.conf"), &fixture.options)
			if err != nil {
				t.Fatal(err)
			}

			var build1Buffer bytes.Buffer
			if err := Build(&build1Buffer, origPayload.Config[0], &BuildOptions{}); err != nil {
				t.Fatal(err)
			}
			build1File := filepath.Join(tmpdir, "build1.conf")
			build1Config := build1Buffer.Bytes()
			if err := ioutil.WriteFile(build1File, build1Config, os.ModePerm); err != nil {
				t.Fatal(err)
			}
			build1Payload, err := Parse(build1File, &fixture.options)
			if err != nil {
				t.Fatal(err)
			}

			if !equalPayloads(*origPayload, *build1Payload) {
				b1, _ := json.Marshal(origPayload)
				b2, _ := json.Marshal(build1Payload)
				if string(b1) != string(b2) {
					t.Fatalf("expected: %s\nbut got: %s", b1, b2)
				}
			}

			var build2Buffer bytes.Buffer
			if err := Build(&build2Buffer, build1Payload.Config[0], &BuildOptions{}); err != nil {
				t.Fatal(err)
			}
			build2File := filepath.Join(tmpdir, "build2.conf")
			build2Config := build2Buffer.Bytes()
			if err := ioutil.WriteFile(build2File, build2Config, os.ModePerm); err != nil {
				t.Fatal(err)
			}
			build2Payload, err := Parse(build2File, &fixture.options)
			if err != nil {
				t.Fatal(err)
			}

			if !equalPayloads(*build1Payload, *build2Payload) {
				b1, _ := json.Marshal(build1Payload)
				b2, _ := json.Marshal(build2Payload)
				if string(b1) != string(b2) {
					t.Fatalf("expected: %s\nbut got: %s", b1, b2)
				}
			}
		})
	}
}

func equalPayloads(p1, p2 Payload) bool {
	return p1.Status == p2.Status &&
		equalPayloadErrors(p1.Errors, p2.Errors) &&
		equalPayloadConfigs(p1.Config, p2.Config)
}

func equalPayloadErrors(e1, e2 []PayloadError) bool {
	if len(e1) != len(e2) {
		return false
	}
	for i := 0; i < len(e1); i++ {
		if e1[i].File != e2[i].File ||
			e1[i].Error != e2[i].Error ||
			(e1[i].Line == nil) != (e2[i].Line == nil) ||
			(e1[i].Line != nil && *e1[i].Line != *e2[i].Line) {
			return false
		}
	}
	return true
}

func equalPayloadConfigs(c1, c2 []Config) bool {
	if len(c1) != len(c2) {
		return false
	}
	for i := 0; i < len(c1); i++ {
		if !equalConfigs(c1[i], c2[i]) {
			return false
		}
	}
	return true
}

func equalConfigs(c1, c2 Config) bool {
	return c1.Status == c2.Status &&
		equalConfigErrors(c1.Errors, c2.Errors) &&
		equalBlocks(c1.Parsed, c2.Parsed)
}

func equalConfigErrors(e1, e2 []ConfigError) bool {
	if len(e1) != len(e2) {
		return false
	}
	for i := 0; i < len(e1); i++ {
		if e1[i].Error != e2[i].Error ||
			(e1[i].Line == nil) != (e2[i].Line == nil) ||
			(e1[i].Line != nil && *e1[i].Line != *e2[i].Line) {
			return false
		}
	}
	return true
}

func equalBlocks(b1, b2 []Directive) bool {
	if len(b1) != len(b2) {
		return false
	}
	for i := 0; i < len(b1); i++ {
		if !equalDirectives(b1[i], b2[i]) {
			return false
		}
	}
	return true
}

func equalDirectives(d1, d2 Directive) bool {
	if d1.Directive != d2.Directive ||
		len(d1.Args) != len(d2.Args) ||
		(d1.Includes == nil) != (d2.Includes == nil) ||
		(d1.Block == nil) != (d2.Block == nil) ||
		(d1.Block != nil && !equalBlocks(*d1.Block, *d2.Block)) ||
		(d1.Comment == nil) != (d2.Comment == nil) ||
		(d1.Comment != nil && *d1.Comment != *d2.Comment) {
		return false
	}
	for i := 0; i < len(d1.Args); i++ {
		if enquote(d1.Args[i]) != enquote(d2.Args[i]) {
			return false
		}
	}
	if d1.Includes != nil {
		for i := 0; i < len(*d1.Includes); i++ {
			if (*d1.Includes)[i] != (*d2.Includes)[i] {
				return false
			}
		}
	}
	return true
}
