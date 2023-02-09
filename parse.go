package crossplane

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var dfltFileOpen = func(path string) (io.Reader, error) { return os.Open(path) }

var hasMagic = regexp.MustCompile(`[*?[]`)

type blockCtx []string

func (c blockCtx) key() string {
	return strings.Join(c, ">")
}

type fileCtx struct {
	path string
	ctx  blockCtx
}

type parser struct {
	configDir   string
	options     *ParseOptions
	handleError func(*Config, error)
	includes    []fileCtx
	included    map[string]int
}

// ParseOptions determine the behavior of an NGINX config parse.
type ParseOptions struct {
	// If true, parsing will stop immediately if an error is found.
	StopParsingOnError bool

	// An array of directives to skip over and not include in the payload.
	IgnoreDirectives []string

	// If true, include directives are used to combine all of the Payload's
	// Config structs into one.
	CombineConfigs bool

	// If true, only the config file with the given filename will be parsed
	// and Parse will not parse files included files.
	SingleFile bool

	// If true, comments will be parsed and added to the resulting Payload.
	ParseComments bool

	// If true, add an error to the payload when encountering a directive that
	// is unrecognized. The unrecognized directive will not be included in the
	// resulting Payload.
	ErrorOnUnknownDirectives bool

	// If true, checks that directives are in valid contexts.
	SkipDirectiveContextCheck bool

	// If true, checks that directives have a valid number of arguments.
	SkipDirectiveArgsCheck bool

	// If an error is found while parsing, it will be passed to this callback
	// function. The results of the callback function will be set in the
	// PayloadError struct that's added to the Payload struct's Errors array.
	ErrorCallback func(error) interface{}

	// If specified, use this alternative to open config files
	Open func(path string) (io.Reader, error)

	// If true, dump copious debugging output for tracing the parsing process.
	Debug bool
}

// Parse parses an NGINX configuration file.
func Parse(filename string, options *ParseOptions) (*Payload, error) {
	payload := Payload{
		Status: "ok",
		Errors: []PayloadError{},
		Config: []Config{},
	}

	handleError := func(config *Config, err error) {
		var line *int
		if e, ok := err.(ParseError); ok {
			line = e.line
		}

		cerr := ConfigError{Line: line, Error: err.Error()}
		perr := PayloadError{Line: line, Error: err.Error(), File: config.File}
		if options.ErrorCallback != nil {
			perr.Callback = options.ErrorCallback(err)
		}

		config.Status = "failed"
		config.Errors = append(config.Errors, cerr)

		payload.Status = "failed"
		payload.Errors = append(payload.Errors, perr)
	}

	// Start with the main nginx config file/context.
	p := parser{
		configDir:   filepath.Dir(filename),
		options:     options,
		handleError: handleError,
		includes:    []fileCtx{fileCtx{path: filename, ctx: blockCtx{}}},
		included:    map[string]int{filename: 0},
	}

	fileOpen := dfltFileOpen
	if options.Open != nil {
		fileOpen = options.Open
	}

	for len(p.includes) > 0 {
		incl := p.includes[0]
		p.includes = p.includes[1:]

		file, err := fileOpen(incl.path)
		if err != nil {
			return nil, err
		}

		tokens := lex(file)
		config := Config{
			File:   incl.path,
			Status: "ok",
			Errors: []ConfigError{},
			Parsed: []Directive{},
		}
		parsed, err := p.parse(&config, tokens, incl.ctx, false)
		if err != nil {
			if options.StopParsingOnError {
				return nil, err
			}
			handleError(&config, err)
		} else {
			config.Parsed = parsed
		}

		payload.Config = append(payload.Config, config)
	}

	if options.CombineConfigs {
		return payload.Combined()
	}

	return &payload, nil
}

// parse Recursively parses directives from an nginx config context.
func (p *parser) parse(parsing *Config, tokens chan ngxToken, ctx blockCtx, consume bool) ([]Directive, error) {
	parsed := []Directive{}

	// parse recursively by pulling from a flat stream of tokens
	for t := range tokens {
		if p.options.Debug {
			fmt.Printf("t (top) Value = '%s', IsQuoted = '%v', Error = '%v'\n", t.Value, t.IsQuoted, t.Error)
		}

		if t.Error != nil {
			return nil, t.Error
		}

		commentsInArgs := []string{}

		// we are parsing a block, so break if it's closing
		if t.Value == "}" && !t.IsQuoted {
			break
		}

		// if we are consuming, then just continue until end of context
		if consume {
			// if we find a block inside this context, consume it too
			if t.Value == "{" && !t.IsQuoted {
				_, _ = p.parse(parsing, tokens, nil, true)
			}
			continue
		}

		// TODO: add a "File" key if combine is true
		// the first token should always be an nginx directive
		stmt := Directive{
			Directive: t.Value,
			Line:      t.Line,
			Args:      []string{},
		}

		// if token is comment
		if strings.HasPrefix(t.Value, "#") && !t.IsQuoted {
			if p.options.ParseComments {
				comment := t.Value[1:]
				stmt.Directive = "#"
				stmt.Comment = &comment
				parsed = append(parsed, stmt)
			}
			continue
		}

		// parse arguments by reading tokens
		t = <-tokens
		for t.IsQuoted || (t.Value != "{" && t.Value != ";" && t.Value != "}") {
			if p.options.Debug {
				fmt.Printf("t (args) Value = '%s', IsQuoted = '%v', Error = '%v'\n", t.Value, t.IsQuoted, t.Error)
			}

			if strings.HasPrefix(t.Value, "#") && !t.IsQuoted {
				commentsInArgs = append(commentsInArgs, t.Value[1:])
			} else {
				stmt.Args = append(stmt.Args, t.Value)
			}
			t = <-tokens
		}
		if p.options.Debug {
			fmt.Printf("t (after args) Value = '%s', IsQuoted = '%v', Error = '%v'\n", t.Value, t.IsQuoted, t.Error)
		}

		// consume the directive if it is ignored and move on
		if contains(p.options.IgnoreDirectives, stmt.Directive) {
			// if this directive was a block consume it too
			if t.Value == "{" && !t.IsQuoted {
				_, _ = p.parse(parsing, tokens, nil, true)
			}
			continue
		}

		// prepare arguments
		if stmt.Directive == "if" {
			stmt = prepareIfArgs(stmt)
		}

		// raise errors if this statement is invalid
		err := analyze(parsing.File, stmt, t.Value, ctx, p.options)

		if perr, ok := err.(ParseError); ok && !p.options.StopParsingOnError {
			p.handleError(parsing, perr)
			// if it was a block but shouldn"t have been then consume
			if strings.HasSuffix(perr.what, ` is not terminated by ";"`) {
				if t.Value != "}" && !t.IsQuoted {
					_, _ = p.parse(parsing, tokens, nil, true)
				} else {
					break
				}
			}
			// keep on parsin'
			continue
		} else if err != nil {
			return nil, err
		}

		// add "includes" to the payload if this is an include statement
		if !p.options.SingleFile && stmt.Directive == "include" {
			pattern := stmt.Args[0]
			if !filepath.IsAbs(pattern) {
				pattern = filepath.Join(p.configDir, pattern)
			}

			stmt.Includes = &[]int{}

			// get names of all included files
			var fnames []string
			if hasMagic.MatchString(pattern) {
				fnames, err = filepath.Glob(pattern)
				if err != nil {
					return nil, err
				}
				sort.Strings(fnames)
			} else {
				// if the file pattern was explicit, nginx will check
				// that the included file can be opened and read
				if f, err := os.Open(pattern); err != nil {
					perr := ParseError{
						what: err.Error(),
						file: &parsing.File,
						line: &stmt.Line,
					}
					if !p.options.StopParsingOnError {
						p.handleError(parsing, perr)
					} else {
						return nil, perr
					}
				} else {
					f.Close()
					fnames = []string{pattern}
				}
			}

			for _, fname := range fnames {
				// the included set keeps files from being parsed twice
				// TODO: handle files included from multiple contexts
				if _, ok := p.included[fname]; !ok {
					p.included[fname] = len(p.included)
					p.includes = append(p.includes, fileCtx{fname, ctx})
				}
				*stmt.Includes = append(*stmt.Includes, p.included[fname])
			}
		}

		// if this statement terminated with "{" then it is a block
		if t.Value == "{" && !t.IsQuoted {
			if p.options.Debug {
				fmt.Println("recurse")
			}
			inner := enterBlockCtx(stmt, ctx) // get context for block

			if strings.HasSuffix(stmt.Directive, "_by_lua_block") {
				// Just consume the lua block contents for now:
				if p.options.Debug {
					fmt.Println("consume")
				}
				_, _ = p.parse(parsing, tokens, inner, true)

			} else {
				if p.options.Debug {
					fmt.Println("parse")
				}
				block, err := p.parse(parsing, tokens, inner, false)
				if err != nil {
					return nil, err
				}
				stmt.Block = &block
			}
			if p.options.Debug {
				fmt.Println("recurse pop")
			}

		}

		parsed = append(parsed, stmt)

		// add all comments found inside args after stmt is added
		for _, comment := range commentsInArgs {
			comment := comment
			parsed = append(parsed, Directive{
				Directive: "#",
				Line:      stmt.Line,
				Args:      []string{},
				Comment:   &comment,
			})
		}
	}

	return parsed, nil
}
