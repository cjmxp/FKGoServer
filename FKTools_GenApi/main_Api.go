//---------------------------------------------
package main

//---------------------------------------------
import (
	BUFIO "bufio"
	BYTES "bytes"
	IO "io"
	IOUTIL "io/ioutil"
	LOG "log"
	OS "os"
	REGEXP "regexp"
	STRCONV "strconv"
	STRINGS "strings"
	TEMPLATE "text/template"
	UNICODE "unicode"

	CLI "gopkg.in/urfave/cli.v2"
)

//---------------------------------------------
const (
	TK_TYPE = iota
	TK_NAME
	TK_PAYLOAD
	TK_COLON
	TK_STRING
	TK_NUMBER
	TK_EOF
	TK_DESC
)

//---------------------------------------------
var (
	keywords = map[string]int{
		"packet_type": TK_TYPE,
		"name":        TK_NAME,
		"payload":     TK_PAYLOAD,
		"desc":        TK_DESC,
	}
)

//---------------------------------------------
var (
	TOKEN_EOF   = &token{typ: TK_EOF}
	TOKEN_COLON = &token{typ: TK_COLON}
)

//---------------------------------------------
type api_expr struct {
	PacketType int
	Name       string
	Payload    string
	Desc       string
}

//---------------------------------------------
type token struct {
	typ     int
	literal string
	number  int
}

//---------------------------------------------
func syntax_error(p *Parser) {
	LOG.Println("syntax error @line:", p.lexer.lineno)
	LOG.Println(">> \033[1;31m", p.lexer.lines[p.lexer.lineno-1], "\033[0m <<")
	OS.Exit(-1)
}

//---------------------------------------------
type Lexer struct {
	reader *BYTES.Buffer
	lines  []string
	lineno int
}

//---------------------------------------------
func (lex *Lexer) init(r IO.Reader) {
	bts, err := IOUTIL.ReadAll(r)
	if err != nil {
		LOG.Fatal(err)
	}

	// 按行读入源码
	scanner := BUFIO.NewScanner(BYTES.NewBuffer(bts))
	for scanner.Scan() {
		lex.lines = append(lex.lines, scanner.Text())
	}

	// 清除注释
	re := REGEXP.MustCompile("(?m:^#(.*)$)")
	bts = re.ReplaceAllLiteral(bts, nil)
	lex.reader = BYTES.NewBuffer(bts)
	lex.lineno = 1
}

//---------------------------------------------
func (lex *Lexer) read_desc() string {
	var runes []rune
	for {
		r, _, err := lex.reader.ReadRune()
		if err == IO.EOF {
			break
		} else if r == '\r' {
			break
		} else if r == '\n' {
			lex.lineno++
			break
		} else {
			runes = append(runes, r)
		}
	}

	return string(runes)
}

//---------------------------------------------
func (lex *Lexer) eof() bool {
	for {
		r, _, err := lex.reader.ReadRune()
		if err == IO.EOF {
			return true
		} else if UNICODE.IsSpace(r) {
			if r == '\n' {
				lex.lineno++
			}
			continue
		} else {
			lex.reader.UnreadRune()
			return false
		}
	}
}

//---------------------------------------------
func (lex *Lexer) next() (t *token) {
	defer func() {
		//	log.Println(t)
	}()
	var r rune
	var err error
	for {
		r, _, err = lex.reader.ReadRune()
		if err == IO.EOF {
			return TOKEN_EOF
		} else if UNICODE.IsSpace(r) {
			if r == '\n' {
				lex.lineno++
			}
			continue
		}
		break
	}

	var runes []rune
	if UNICODE.IsLetter(r) {
		for {
			runes = append(runes, r)
			r, _, err = lex.reader.ReadRune()
			if err == IO.EOF {
				break
			} else if UNICODE.IsLetter(r) || UNICODE.IsNumber(r) || r == '_' {
				continue
			} else {
				lex.reader.UnreadRune()
				break
			}
		}
		t := &token{}
		if tkid, ok := keywords[string(runes)]; ok {
			t.typ = tkid
		} else {
			t.typ = TK_STRING
			t.literal = string(runes)
		}
		return t
	} else if UNICODE.IsNumber(r) {
		for {
			runes = append(runes, r)
			r, _, err = lex.reader.ReadRune()
			if err == IO.EOF {
				break
			} else if UNICODE.IsNumber(r) {
				continue
			} else {
				lex.reader.UnreadRune()
				break
			}
		}
		t := &token{}
		t.typ = TK_NUMBER
		n, _ := STRCONV.Atoi(string(runes))
		t.number = n
		return t
	} else if r == ':' {
		return TOKEN_COLON
	} else {
		LOG.Fatal("lex error @line:", lex.lineno)
	}
	return nil
}

//////////////////////////////////////////////////////////////
type Parser struct {
	exprs []api_expr
	lexer *Lexer
}

//---------------------------------------------
func (p *Parser) init(lex *Lexer) {
	p.lexer = lex
}

//---------------------------------------------
func (p *Parser) match(typ int) *token {
	t := p.lexer.next()
	if t.typ != typ {
		syntax_error(p)
	}
	return t
}

//---------------------------------------------
func (p *Parser) expr() bool {
	if p.lexer.eof() {
		return false
	}
	api := api_expr{}

	p.match(TK_TYPE)
	p.match(TK_COLON)
	t := p.match(TK_NUMBER)
	api.PacketType = t.number

	p.match(TK_NAME)
	p.match(TK_COLON)
	t = p.match(TK_STRING)
	api.Name = t.literal

	p.match(TK_PAYLOAD)
	p.match(TK_COLON)
	t = p.match(TK_STRING)
	api.Payload = t.literal

	p.match(TK_DESC)
	p.match(TK_COLON)
	api.Desc = p.lexer.read_desc()

	p.exprs = append(p.exprs, api)
	return true
}

//---------------------------------------------
func main() {
	app := &CLI.App{
		Name:  "Protocol Handler Generator",
		Usage: "handle api.txt",
		Authors: []CLI.Author{
			{Name: "xtaci"},
			{Name: "ycs"},
		},
		Version: "1.1",
		Flags: []CLI.Flag{
			&CLI.StringFlag{Name: "file,f", Value: "./api.txt", Usage: "input api.txt file"},
			&CLI.IntFlag{Name: "min_proto,min", Value: 0, Usage: "minimum proto number"},
			&CLI.IntFlag{Name: "max_proto,max", Value: 1000, Usage: "maximum proto number"},
			&CLI.StringFlag{Name: "template,t", Value: "./templates/server/api.tmpl", Usage: "template file"},
			&CLI.StringFlag{Name: "pkgname", Value: "agent", Usage: "package name to prefix"},
		},
		Action: func(c *CLI.Context) error {
			// parse
			file, err := OS.Open(c.String("file"))
			if err != nil {
				LOG.Fatal(err)
			}
			lexer := Lexer{}
			lexer.init(file)
			p := Parser{}
			p.init(&lexer)
			for p.expr() {
			}

			// use template to generate final output
			funcMap := TEMPLATE.FuncMap{
				"isReq": func(api api_expr) bool {
					if api.PacketType < c.Int("min_proto") || api.PacketType > c.Int("max_proto") {
						return false
					}
					if STRINGS.HasSuffix(api.Name, "_req") {
						return true
					}
					return false
				},
			}
			tmpl, err := TEMPLATE.New("api.tmpl").Funcs(funcMap).ParseFiles(c.String("template"))
			if err != nil {
				LOG.Fatal(err)
			}
			args := struct {
				PackageName string
				Exprs       []api_expr
			}{c.String("pkgname"), p.exprs}

			err = tmpl.Execute(OS.Stdout, args)
			if err != nil {
				LOG.Fatal(err)
			}
			return nil
		},
	}
	app.Run(OS.Args)
}
