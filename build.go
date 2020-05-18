package crossplane

import (
	"io"
	"strings"
)

type BuildOptions struct {
	Indent int
	Tabs   bool
	Header bool
}

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

	// arguments can't start with variable expansion syntax
	char = <-chars
	if isSpace(char) || char == "{" || char == "}" || char == ";" || char == `"` || char == "'" || char == "${" {
		return true
	}

	expanding := false
	for c := range chars {
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

func escape(s string) chan string {
	c := make(chan string)
	go func() {
		prev, char := "", ""
		for _, r := range s {
			char = string(r)
			if prev == "\\" || prev+char == "${" {
				prev += char
				c <- prev
				continue
			}
			if prev == "$" {
				c <- prev
			}
			if char != "\\" && char != "$" {
				c <- char
			}
			prev = char
		}
		if char == "\\" || char == "$" {
			c <- char
		}
		close(c)
	}()
	return c
}
