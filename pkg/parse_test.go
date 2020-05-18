package crossplane

import (
	"encoding/json"
	"log"
	"path/filepath"
	"testing"
)

type parseFixture struct {
	name     string
	suffix   string
	options  ParseOptions
	expected Payload
}

func pInt(i int) *int {
	return &i
}

func pStr(s string) *string {
	return &s
}

var parseFixtures = []parseFixture{
	parseFixture{"includes-regular", "", ParseOptions{}, Payload{
		Status: "failed",
		Errors: []PayloadError{
			PayloadError{
				File:  "../test/testdata/includes-regular/conf.d/server.conf",
				Error: "open ../test/testdata/includes-regular/bar.conf: no such file or directory in ../test/testdata/includes-regular/conf.d/server.conf:5",
				Line:  pInt(5),
			},
		},
		Config: []Config{
			Config{
				File:   "../test/testdata/includes-regular/nginx.conf",
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
						Block: &[]Directive{
							Directive{
								Directive: "include",
								Args:      []string{"conf.d/server.conf"},
								Line:      3,
								Includes:  &[]int{1},
							},
						},
					},
				},
			},
			Config{
				File:   "../test/testdata/includes-regular/conf.d/server.conf",
				Status: "failed",
				Errors: []ConfigError{
					ConfigError{
						Error: "open ../test/testdata/includes-regular/bar.conf: no such file or directory in ../test/testdata/includes-regular/conf.d/server.conf:5",
						Line:  pInt(5),
					},
				},
				Parsed: []Directive{
					Directive{
						Directive: "server",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "listen",
								Args:      []string{"127.0.0.1:8080"},
								Line:      2,
							},
							Directive{
								Directive: "server_name",
								Args:      []string{"default_server"},
								Line:      3,
							},
							Directive{
								Directive: "include",
								Args:      []string{"foo.conf"},
								Line:      4,
								Includes:  &[]int{2},
							},
							Directive{
								Directive: "include",
								Args:      []string{"bar.conf"},
								Line:      5,
								Includes:  &[]int{},
							},
						},
					},
				},
			},
			Config{
				File:   "../test/testdata/includes-regular/foo.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "location",
						Args:      []string{"/foo"},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "return",
								Args:      []string{"200", "foo"},
								Line:      2,
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"includes-regular", "-single-file", ParseOptions{SingleFile: true}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/includes-regular/nginx.conf",
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
						Block: &[]Directive{
							Directive{
								Directive: "include",
								Args:      []string{"conf.d/server.conf"},
								Line:      3,
								// no Includes key
							},
						},
					},
				},
			},
			// single config parsed
		},
	}},
	parseFixture{"includes-globbed", "", ParseOptions{}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/includes-globbed/nginx.conf",
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
						Directive: "include",
						Args:      []string{"http.conf"},
						Line:      2,
						Includes:  &[]int{1},
					},
				},
			},
			Config{
				File:   "../test/testdata/includes-globbed/http.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					{
						Directive: "http",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							{
								Directive: "include",
								Args:      []string{"servers/*.conf"},
								Line:      2,
								Includes:  &[]int{2, 3},
							},
						},
					},
				},
			},
			Config{
				File:   "../test/testdata/includes-globbed/servers/server1.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					{
						Directive: "server",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							{
								Directive: "listen",
								Args:      []string{"8080"},
								Line:      2,
							},
							{
								Directive: "include",
								Args:      []string{"locations/*.conf"},
								Line:      3,
								Includes:  &[]int{4, 5},
							},
						},
					},
				},
			},
			Config{
				File:   "../test/testdata/includes-globbed/servers/server2.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					{
						Directive: "server",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "listen",
								Args:      []string{"8081"},
								Line:      2,
							},
							Directive{
								Directive: "include",
								Args:      []string{"locations/*.conf"},
								Line:      3,
								Includes:  &[]int{4, 5},
							},
						},
					},
				},
			},
			Config{
				File:   "../test/testdata/includes-globbed/locations/location1.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "location",
						Args:      []string{"/foo"},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "return",
								Args:      []string{"200", "foo"},
								Line:      2,
							},
						},
					},
				},
			},
			Config{
				File:   "../test/testdata/includes-globbed/locations/location2.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "location",
						Args:      []string{"/bar"},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "return",
								Args:      []string{"200", "bar"},
								Line:      2,
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"includes-globbed", "-combine-configs", ParseOptions{CombineConfigs: true}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/includes-globbed/nginx.conf",
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
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      1,
								Block: &[]Directive{
									Directive{
										Directive: "listen",
										Args:      []string{"8080"},
										Line:      2,
									},
									Directive{
										Directive: "location",
										Args:      []string{"/foo"},
										Line:      1,
										Block: &[]Directive{
											Directive{
												Directive: "return",
												Args:      []string{"200", "foo"},
												Line:      2,
											},
										},
									},
									Directive{
										Directive: "location",
										Args:      []string{"/bar"},
										Line:      1,
										Block: &[]Directive{
											Directive{
												Directive: "return",
												Args:      []string{"200", "bar"},
												Line:      2,
											},
										},
									},
								},
							},
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      1,
								Block: &[]Directive{
									Directive{
										Directive: "listen",
										Args:      []string{"8081"},
										Line:      2,
									},
									Directive{
										Directive: "location",
										Args:      []string{"/foo"},
										Line:      1,
										Block: &[]Directive{
											Directive{
												Directive: "return",
												Args:      []string{"200", "foo"},
												Line:      2,
											},
										},
									},
									Directive{
										Directive: "location",
										Args:      []string{"/bar"},
										Line:      1,
										Block: &[]Directive{
											Directive{
												Directive: "return",
												Args:      []string{"200", "bar"},
												// File:      "../test/testdata//locations/location2.conf",
												Line: 2,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"simple", "-ignore-directives-1", ParseOptions{IgnoreDirectives: []string{"listen", "server_name"}}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/simple/nginx.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "events",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "worker_connections",
								Args:      []string{"1024"},
								Line:      2,
							},
						},
					},
					Directive{
						Directive: "http",
						Args:      []string{},
						Line:      5,
						Block: &[]Directive{
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      6,
								Block: &[]Directive{
									Directive{
										Directive: "location",
										Args:      []string{"/"},
										Line:      9,
										Block: &[]Directive{
											Directive{
												Directive: "return",
												Args:      []string{"200", "foo bar baz"},
												Line:      10,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"simple", "-ignore-directives-2", ParseOptions{IgnoreDirectives: []string{"events", "server"}}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/simple/nginx.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "http",
						Args:      []string{},
						Line:      5,
						Block:     &[]Directive{},
					},
				},
			},
		},
	}},
	parseFixture{"with-comments", "-true", ParseOptions{ParseComments: true}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/with-comments/nginx.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "events",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "worker_connections",
								Args:      []string{"1024"},
								Line:      2,
							},
						},
					},
					Directive{
						Directive: "#",
						Args:      []string{},
						Line:      4,
						Comment:   pStr("comment"),
					},
					Directive{
						Directive: "http",
						Args:      []string{},
						Line:      5,
						Block: &[]Directive{
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      6,
								Block: &[]Directive{
									Directive{
										Directive: "listen",
										Args:      []string{"127.0.0.1:8080"},
										Line:      7,
									},
									Directive{
										Directive: "#",
										Args:      []string{},
										Line:      7,
										Comment:   pStr("listen"),
									},
									Directive{
										Directive: "server_name",
										Args:      []string{"default_server"},
										Line:      8,
									},
									Directive{
										Directive: "location",
										Args:      []string{"/"},
										Line:      9,
										Block: &[]Directive{
											Directive{
												Directive: "#",
												Args:      []string{},
												Line:      9,
												Comment:   pStr("# this is brace"),
											},
											Directive{
												Directive: "#",
												Args:      []string{},
												Line:      10,
												Comment:   pStr(" location /"),
											},
											Directive{
												Directive: "return",
												Args:      []string{"200", "foo bar baz"},
												Line:      11,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"with-comments", "-false", ParseOptions{ParseComments: false}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/with-comments/nginx.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "events",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "worker_connections",
								Args:      []string{"1024"},
								Line:      2,
							},
						},
					},
					Directive{
						Directive: "http",
						Args:      []string{},
						Line:      5,
						Block: &[]Directive{
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      6,
								Block: &[]Directive{
									Directive{
										Directive: "listen",
										Args:      []string{"127.0.0.1:8080"},
										Line:      7,
									},
									Directive{
										Directive: "server_name",
										Args:      []string{"default_server"},
										Line:      8,
									},
									Directive{
										Directive: "location",
										Args:      []string{"/"},
										Line:      9,
										Block: &[]Directive{
											Directive{
												Directive: "return",
												Args:      []string{"200", "foo bar baz"},
												Line:      11,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"spelling-mistake", "", ParseOptions{ParseComments: true, ErrorOnUnknownDirectives: true}, Payload{
		Status: "failed",
		Errors: []PayloadError{
			PayloadError{
				File:  "../test/testdata/spelling-mistake/nginx.conf",
				Error: `unknown directive "proxy_passs" in ../test/testdata/spelling-mistake/nginx.conf:7`,
				Line:  pInt(7),
			},
		},
		Config: []Config{
			Config{
				File:   "../test/testdata/spelling-mistake/nginx.conf",
				Status: "failed",
				Errors: []ConfigError{
					ConfigError{
						Error: `unknown directive "proxy_passs" in ../test/testdata/spelling-mistake/nginx.conf:7`,
						Line:  pInt(7),
					},
				},
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
						Line:      3,
						Block: &[]Directive{
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      4,
								Block: &[]Directive{
									Directive{
										Directive: "location",
										Args:      []string{"/"},
										Line:      5,
										Block: &[]Directive{
											Directive{
												Directive: "#",
												Args:      []string{},
												Line:      6,
												Comment:   pStr("directive is misspelled"),
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"missing-semicolon-above", "", ParseOptions{}, Payload{
		Status: "failed",
		Errors: []PayloadError{
			PayloadError{
				File:  "../test/testdata/missing-semicolon-above/nginx.conf",
				Error: "directive \"proxy_pass\" is not terminated by \";\" in ../test/testdata/missing-semicolon-above/nginx.conf:4",
				Line:  pInt(4),
			},
		},
		Config: []Config{
			Config{
				File:   "../test/testdata/missing-semicolon-above/nginx.conf",
				Status: "failed",
				Errors: []ConfigError{
					ConfigError{
						Error: "directive \"proxy_pass\" is not terminated by \";\" in ../test/testdata/missing-semicolon-above/nginx.conf:4",
						Line:  pInt(4),
					},
				},
				Parsed: []Directive{
					Directive{
						Directive: "http",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      2,
								Block: &[]Directive{
									Directive{
										Directive: "location",
										Args:      []string{"/is-broken"},
										Line:      3,
										Block:     &[]Directive{},
									},
									Directive{
										Directive: "location",
										Args:      []string{"/not-broken"},
										Line:      6,
										Block: &[]Directive{
											Directive{
												Directive: "proxy_pass",
												Args:      []string{"http://not.broken.example"},
												Line:      7,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"missing-semicolon-below", "", ParseOptions{}, Payload{
		Status: "failed",
		Errors: []PayloadError{
			PayloadError{
				File:  "../test/testdata/missing-semicolon-below/nginx.conf",
				Error: "directive \"proxy_pass\" is not terminated by \";\" in ../test/testdata/missing-semicolon-below/nginx.conf:7",
				Line:  pInt(7),
			},
		},
		Config: []Config{
			Config{
				File:   "../test/testdata/missing-semicolon-below/nginx.conf",
				Status: "failed",
				Errors: []ConfigError{
					ConfigError{
						Error: "directive \"proxy_pass\" is not terminated by \";\" in ../test/testdata/missing-semicolon-below/nginx.conf:7",
						Line:  pInt(7),
					},
				},
				Parsed: []Directive{
					Directive{
						Directive: "http",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "server",
								Args:      []string{},
								Line:      2,
								Block: &[]Directive{
									Directive{
										Directive: "location",
										Args:      []string{"/not-broken"},
										Line:      3,
										Block: &[]Directive{
											Directive{
												Directive: "proxy_pass",
												Args:      []string{"http://not.broken.example"},
												Line:      4,
											},
										},
									},
									Directive{
										Directive: "location",
										Args:      []string{"/is-broken"},
										Line:      6,
										Block:     &[]Directive{},
									},
								},
							},
						},
					},
				},
			},
		},
	}},
	parseFixture{"comments-between-args", "", ParseOptions{ParseComments: true}, Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{
			Config{
				File:   "../test/testdata/comments-between-args/nginx.conf",
				Status: "ok",
				Errors: []ConfigError{},
				Parsed: []Directive{
					Directive{
						Directive: "http",
						Args:      []string{},
						Line:      1,
						Block: &[]Directive{
							Directive{
								Directive: "#",
								Args:      []string{},
								Line:      1,
								Comment:   pStr("comment 1"),
							},
							Directive{
								Directive: "log_format",
								Args:      []string{"\\#arg\\ 1", "#arg 2"},
								Line:      2,
							},
							Directive{
								Directive: "#",
								Args:      []string{},
								Line:      2,
								Comment:   pStr("comment 2"),
							},
							Directive{
								Directive: "#",
								Args:      []string{},
								Line:      2,
								Comment:   pStr("comment 3"),
							},
							Directive{
								Directive: "#",
								Args:      []string{},
								Line:      2,
								Comment:   pStr("comment 4"),
							},
							Directive{
								Directive: "#",
								Args:      []string{},
								Line:      2,
								Comment:   pStr("comment 5"),
							},
						},
					},
				},
			},
		},
	}},
}

func TestParse(t *testing.T) {
	for _, fixture := range parseFixtures {
		t.Run(fixture.name+fixture.suffix, func(t *testing.T) {
			path := filepath.Join("../test/testdata", fixture.name, "nginx.conf")
			payload, err := Parse(path, &fixture.options)
			if err != nil {
				log.Fatal(err)
			}
			b1, _ := json.Marshal(fixture.expected)
			b2, _ := json.Marshal(payload)
			if string(b1) != string(b2) {
				log.Fatalf("expected: %s\nbut got: %s", b1, b2)
			}
		})
	}
}
