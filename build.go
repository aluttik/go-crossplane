package crossplane

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

type BuildOptions struct {
	Indent int
	Tabs   bool
	Header bool
}

// BuildFiles builds all of the config files in a crossplane.Payload and
// writes them to disk.
func BuildFiles(payload Payload, dir string, options *BuildOptions) error {
	if len(dir) == 0 {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		dir = cwd
	}

	for _, config := range payload.Config {
		path := config.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}

		// make directories that need to be made for the config to be built
		dirpath := filepath.Dir(path)
		if err := os.MkdirAll(dirpath, os.ModeDir|os.ModePerm); err != nil {
			return err
		}

		// build then create the nginx config file using the json payload
		var buf bytes.Buffer
		if err := Build(&buf, config, options); err != nil {
			return err
		}

		f, err := os.Create(path)
		if err != nil {
			return err
		}
		defer f.Close()

		output := append(bytes.TrimRightFunc(buf.Bytes(), unicode.IsSpace), '\n')
		if _, err := f.Write(output); err != nil {
			return err
		}
	}

	return nil
}

// Build creates an NGINX config from a crossplane.Config.
func Build(w io.Writer, config Config, options *BuildOptions) error {
	if options.Indent == 0 {
		options.Indent = 4
	}

	head := ""
	if options.Header {
		head += "# This config was built from JSON using NGINX crossplane.\n"
		head += "# If you encounter any bugs please report them here:\n"
		head += "# https://github.com/nginxinc/crossplane/issues\n"
		head += "\n"
	}

	body := ""
	body = buildBlock(body, config.Parsed, 0, 0, options)
	_, err := w.Write([]byte(head + body))
	return err
}

func buildBlock(output string, block []Directive, depth int, lastLine int, options *BuildOptions) string {
	for _, stmt := range block {
		var built string

		if stmt.IsComment() && stmt.Line == lastLine {
			output += " #" + *stmt.Comment
			continue
		} else if stmt.IsComment() {
			built = "#" + *stmt.Comment
		} else {
			directive := enquote(stmt.Directive)
			args := []string{}
			for _, arg := range stmt.Args {
				args = append(args, enquote(arg))
			}

			if directive == "if" {
				built = "if (" + strings.Join(args, " ") + ")"
			} else if len(args) > 0 {
				built = directive + " " + strings.Join(args, " ")
			} else {
				built = directive
			}

			if stmt.Block == nil {
				built += ";"
			} else {
				built += " {"
				built = buildBlock(built, *stmt.Block, depth+1, stmt.Line, options)
				built += "\n" + margin(options, depth) + "}"
			}
		}
		if len(output) > 0 {
			output += "\n"
		}
		output += margin(options, depth) + built
		lastLine = stmt.Line
	}

	return output
}

func margin(options *BuildOptions, depth int) string {
	if options.Tabs {
		return strings.Repeat("\t", depth)
	}
	return strings.Repeat(" ", options.Indent*depth)
}

func enquote(arg string) string {
	if !needsQuotes(arg) {
		return arg
	}
	quoted := strings.ReplaceAll(repr(arg), `\\`, `\`)
	return quoted
}

func needsQuotes(s string) bool {
	if s == "" {
		return true
	}

	// lexer should throw an error when variable expansion syntax
	// is messed up, but just wrap it in quotes for now I guess
	var char string
	chars := escape(s)

	if len(chars) == 0 {
		return true
	}

	// arguments can't start with variable expansion syntax
	char = chars[0]
	chars = chars[1:]

	if isSpace(char) || char == "{" || char == "}" || char == ";" || char == `"` || char == "'" || char == "${" {
		return true
	}

	expanding := false
	for _, c := range chars {
		char = c
		if isSpace(char) || char == "{" || char == ";" || char == `"` || char == "'" {
			return true
		} else if (expanding && char == "${") || (!expanding && char == "}") {
			return true
		} else if (expanding && char == "}") || (!expanding && char == "${") {
			expanding = !expanding
		}
	}

	return expanding || char == "\\" || char == "$"
}

func escape(s string) []string {
	var c []string
	prev, char := "", ""
	for _, r := range s {
		char = string(r)
		if prev == "\\" || prev+char == "${" {
			prev += char
			c = append(c, prev)
			continue
		}
		if prev == "$" {
			c = append(c, prev)
		}
		if char != "\\" && char != "$" {
			c = append(c, char)
		}
		prev = char
	}
	if char == "\\" || char == "$" {
		c = append(c, char)
	}

	return c
}
