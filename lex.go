package crossplane

import (
	"bufio"
	"io"
	"strings"
)

type Token struct {
	Value    string
	Line     int
	IsQuoted bool
	Error    error
}

type charLine struct {
	char string
	line int
}

//
func Lex(reader io.Reader) chan Token {
	return balanceBraces(lex(reader))
}

func balanceBraces(tokens chan Token) chan Token {
	c := make(chan Token)

	go func() {
		depth := 0
		line := 0
		for t := range tokens {
			line = t.Line
			if t.Value == "}" && !t.IsQuoted {
				depth--
			} else if t.Value == "{" && !t.IsQuoted {
				depth++
			}

			// raise error if we ever have more right braces than left
			if depth < 0 {
				c <- Token{
					Error: ParseError{
						what: `unexpected "}"`,
						line: &line,
					},
				}
				close(c)
				return
			}
			c <- t
		}

		// raise error if we have less right braces than left at EOF
		if depth > 0 {
			c <- Token{
				Error: ParseError{
					what: `unexpected end of file, expecting "}"`,
					line: &line,
				},
			}
		}

		close(c)
	}()

	return c
}

func lex(reader io.Reader) chan Token {
	c := make(chan Token)

	go func() {
		var ok bool
		var token string
		var tokenLine int

		it := lineCount(escapeChars(readChars(reader)))

		for cl := range it {
			// handle whitespace
			if isSpace(cl.char) {
				// if token complete yield it and reset token buffer
				if len(token) > 0 {
					c <- Token{Value: token, Line: tokenLine, IsQuoted: false}
					token = ""
				}
				// disregard until char isn't a whitespace character
				for isSpace(cl.char) {
					if cl, ok = <-it; !ok {
						break
					}
				}
			}

			// if starting comment
			if len(token) == 0 && cl.char == "#" {
				lineAtStart := cl.line
				for !strings.HasSuffix(cl.char, "\n") {
					token += cl.char
					if cl, ok = <-it; !ok {
						break
					}
				}
				c <- Token{Value: token, Line: lineAtStart, IsQuoted: false}
				token = ""
				continue
			}

			if len(token) == 0 {
				tokenLine = cl.line
			}

			// handle parameter expansion syntax (ex: "${var[@]}")
			if len(token) > 0 && strings.HasSuffix(token, "$") && cl.char == "{" {
				for !strings.HasSuffix(token, "}") && !isSpace(cl.char) {
					token += cl.char
					if cl, ok = <-it; !ok {
						break
					}
				}
			}

			// if a quote is found, add the whole string to the token buffer
			if cl.char == `"` || cl.char == "'" {
				// if a quote is inside a token, treat it like any other char
				if len(token) > 0 {
					token += cl.char
					continue
				}

				quote := cl.char
				if cl, ok = <-it; !ok {
					break
				}
				for cl.char != quote {
					if cl.char == "\\"+quote {
						token += quote
					} else {
						token += cl.char
					}
					if cl, ok = <-it; !ok {
						break
					}
				}

				// True because this is in quotes
				c <- Token{Value: token, Line: tokenLine, IsQuoted: true}
				token = ""
				continue
			}

			// handle special characters that are treated like full tokens
			if cl.char == "{" || cl.char == "}" || cl.char == ";" {
				// if token complete yield it and reset token buffer
				if len(token) > 0 {
					c <- Token{Value: token, Line: tokenLine, IsQuoted: false}
					token = ""
				}

				// this character is a full token so yield it now
				c <- Token{Value: cl.char, Line: cl.line, IsQuoted: false}
				continue
			}

			// append char to the token buffer
			token += cl.char
		}

		if token != "" {
			c <- Token{Value: token, Line: tokenLine, IsQuoted: false}
		}

		close(c)
	}()

	return c
}

func readChars(reader io.Reader) chan string {
	c := make(chan string)

	go func() {
		scanner := bufio.NewScanner(reader)
		scanner.Split(bufio.ScanRunes)
		for scanner.Scan() {
			c <- scanner.Text()
		}
		close(c)
	}()

	return c
}

func lineCount(chars chan string) chan charLine {
	c := make(chan charLine)

	go func() {
		line := 1
		for char := range chars {
			if strings.HasSuffix(char, "\n") {
				line++
			}
			c <- charLine{char: char, line: line}
		}
		close(c)
	}()

	return c
}

func escapeChars(chars chan string) chan string {
	c := make(chan string)

	go func() {
		for char := range chars {
			if char == "\\" {
				char += <-chars
			}
			// Skip carriage return characters.
			if char == "\r" {
				continue
			}
			c <- char
		}
		close(c)
	}()

	return c
}

func isSpace(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}
