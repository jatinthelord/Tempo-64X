// Package parser implements the Tempo language parser.
// It converts a token stream into an AST.
package parser

import (
	"fmt"
	"strings"

	"github.com/jatu/tempo/lexer"
	"github.com/jatu/tempo/token"
)

// ---- AST Node Types ----

// Node is the base interface for all AST nodes
type Node interface {
	nodeType() string
}

// Error node
type Error struct{ Value string }
func (e Error) nodeType() string { return "Error" }

// Label node: name:
type Label struct{ Name string; Line int }
func (l Label) nodeType() string { return "Label" }

// Data node: msg db 'hello', 0x0a
type Data struct {
	Name     string
	Contents []byte
	Line     int
}
func (d Data) nodeType() string { return "Data" }

// Operand sizes
const (
	SizeDefault = 0
	Size8       = 8
	Size16      = 16
	Size32      = 32
	Size64      = 64
)

// Operand holds a single instruction operand
type Operand struct {
	token.Token
	Indirection bool // e.g. [rax]
	Size        int  // 8/16/32/64
}

// Instruction node: mov eax, 1
type Instruction struct {
	Instruction string
	Operands    []Operand
	Line        int
}
func (i Instruction) nodeType() string { return "Instruction" }

// Section node
type Section struct {
	Name string
	Line int
}
func (s Section) nodeType() string { return "Section" }

// AliasDecl: [eax == myReg]
type AliasDecl struct {
	Register string
	Name     string
	Line     int
}
func (a AliasDecl) nodeType() string { return "AliasDecl" }

// Shape2D: [a * z] object
type Shape2D struct {
	Kind   string   // "square", "rect", "line", "custom"
	Width  int
	Height int
	Regs   []string // registers used
	Line   int
}
func (s Shape2D) nodeType() string { return "Shape2D" }

// RmpiExec: %rmpi% execution block
type RmpiExec struct {
	HexBytes []string
	Syscalls []string
	Line     int
}
func (r RmpiExec) nodeType() string { return "RmpiExec" }

// CmdBlock: cmd[]==  block
type CmdBlock struct {
	Operands []string
	Line     int
}
func (c CmdBlock) nodeType() string { return "CmdBlock" }

// PrintShortcut: shorthand print block
type PrintShortcut struct {
	Message string
	Len     string
	Line    int
}
func (p PrintShortcut) nodeType() string { return "PrintShortcut" }

// ---- Parser ----

// Parser holds the parser state
type Parser struct {
	l        *lexer.Lexer
	cur      token.Token
	peek     token.Token
	aliases  map[string]string // alias name -> register
	errors   []string
}

// New creates a new Parser from the given source
func New(src string) *Parser {
	p := &Parser{
		l:       lexer.New(src),
		aliases: make(map[string]string),
	}
	p.advance()
	p.advance()
	return p
}

func (p *Parser) advance() {
	p.cur = p.peek
	for {
		p.peek = p.l.NextToken()
		if p.peek.Type != token.NEWLINE {
			break
		}
	}
}

func (p *Parser) skipNewlines() {
	for p.cur.Type == token.NEWLINE {
		p.advance()
	}
}

// Next returns the next AST node, or nil at EOF
func (p *Parser) Next() Node {
	p.skipNewlines()
	if p.cur.Type == token.EOF {
		return nil
	}

	switch p.cur.Type {

	case token.SECTION_DATA, token.SECTION_TEXT:
		sec := Section{Name: p.cur.Literal, Line: p.cur.Line}
		p.advance()
		return sec

	case token._BEGIN, token._START:
		lbl := Label{Name: p.cur.Literal, Line: p.cur.Line}
		p.advance()
		if p.cur.Type == token.COLON {
			p.advance()
		}
		return lbl

	case token.IDENTIFIER:
		// Could be: label, data decl, or alias
		name := p.cur.Literal
		p.advance()

		// label: NAME:
		if p.cur.Type == token.COLON {
			p.advance()
			return Label{Name: name, Line: p.peek.Line}
		}

		// data: NAME db/dw/dd/dq 'str', 0x0a
		if isDataDirective(p.cur.Type) {
			return p.parseData(name)
		}

		// alias declaration outside bracket: handled in LBRACKET branch
		return Error{Value: fmt.Sprintf("unexpected identifier %q at line %d", name, p.cur.Line)}

	case token.LBRACKET:
		return p.parseBracketExpr()

	case token.RMPI:
		return p.parseRmpiBlock()

	case token.LLTD:
		return p.parseLLtdBlock()

	case token.CMD:
		return p.parseCmdBlock()

	// All instructions
	case token.MOV, token.ADD, token.SUB, token.MUL, token.DIV,
		token.XOR, token.AND, token.OR, token.NOT,
		token.INC, token.DEC, token.PUSH, token.POP,
		token.CALL, token.RET, token.NOP, token.INT, token.SYSCALL, token.HLT,
		token.JNS, token.JMP, token.JNZ, token.JNE, token.JZ, token.JEQ,
		token.CLP, token.CMP:
		return p.parseInstruction()

	case token.JE:
		// je can be an instruction OR object marker; if next is register treat as instruction
		if p.peek.Type == token.REGISTER || p.peek.Type == token.IDENTIFIER {
			return p.parseInstruction()
		}
		p.advance()
		return Section{Name: "je-marker", Line: p.cur.Line}

	case token.NN:
		p.advance()
		return Instruction{Instruction: "nop", Line: p.cur.Line}

	case token.QR:
		p.advance()
		return Section{Name: "qr-end", Line: p.cur.Line}

	case token._GLOBAL:
		p.advance()
		name := ""
		if p.cur.Type == token.IDENTIFIER || p.cur.Type == token.MSG_KEYWORD {
			name = p.cur.Literal
			p.advance()
		}
		return Label{Name: "_global_" + name, Line: p.cur.Line}

	case token.LEN_KEYWORD:
		// len equ $ - msg  (standalone)
		p.advance()
		if p.cur.Type == token.EQU {
			p.advance()
		}
		return Instruction{Instruction: "len_equ", Line: p.cur.Line}

	default:
		tok := p.cur
		p.advance()
		return Error{Value: fmt.Sprintf("unexpected token %q (%s) at line %d", tok.Literal, tok.Type, tok.Line)}
	}
}

func (p *Parser) parseData(name string) Node {
	directive := p.cur.Type
	p.advance()

	contents := []byte{}

	for p.cur.Type != token.NEWLINE && p.cur.Type != token.EOF {
		switch p.cur.Type {
		case token.STRING:
			contents = append(contents, []byte(p.cur.Literal)...)
		case token.HEX_LITERAL:
			var b byte
			fmt.Sscanf(p.cur.Literal, "0x%x", &b)
			if directive == token.DB {
				contents = append(contents, b)
			}
		case token.NUMBER:
			var n byte
			fmt.Sscanf(p.cur.Literal, "%d", &n)
			if directive == token.DB {
				contents = append(contents, n)
			}
		case token.COMMA:
			// separator, skip
		default:
			// skip unknown tokens in data
		}
		p.advance()
	}
	return Data{Name: name, Contents: contents, Line: p.cur.Line}
}

func (p *Parser) parseInstruction() Node {
	instr := strings.ToLower(p.cur.Literal)
	line := p.cur.Line
	p.advance()

	operands := []Operand{}

	for p.cur.Type != token.NEWLINE && p.cur.Type != token.EOF {
		if p.cur.Type == token.COMMA {
			p.advance()
			continue
		}

		op, err := p.parseOperand()
		if err != nil {
			return Error{Value: err.Error()}
		}
		operands = append(operands, op)
	}

	return Instruction{Instruction: instr, Operands: operands, Line: line}
}

func (p *Parser) parseOperand() (Operand, error) {
	op := Operand{}
	op.Size = SizeDefault

	// Size qualifier: byte/word/dword/qword ptr
	switch p.cur.Type {
	case token.BYTE, token.SIZE_BYTE:
		op.Size = Size8
		p.advance()
		if p.cur.Type == token.SIZE_PTR {
			p.advance()
		}
	case token.SIZE_WORD:
		op.Size = Size16
		p.advance()
		if p.cur.Type == token.SIZE_PTR {
			p.advance()
		}
	case token.SIZE_DWORD:
		op.Size = Size32
		p.advance()
		if p.cur.Type == token.SIZE_PTR {
			p.advance()
		}
	case token.SIZE_QWORD:
		op.Size = Size64
		p.advance()
		if p.cur.Type == token.SIZE_PTR {
			p.advance()
		}
	}

	// Indirection: [reg]
	if p.cur.Type == token.LBRACKET {
		op.Indirection = true
		p.advance()
		op.Token = p.cur
		if op.Size == SizeDefault {
			op.Size = Size64
		}
		p.advance()
		// skip closing ]
		if p.cur.Type == token.RBRACKET {
			p.advance()
		}
		return op, nil
	}

	// Register
	if p.cur.Type == token.REGISTER {
		op.Token = p.cur
		// resolve alias
		if resolved, ok := p.aliases[p.cur.Literal]; ok {
			op.Token.Literal = resolved
		}
		p.advance()
		return op, nil
	}

	// Identifier (label / data name)
	if p.cur.Type == token.IDENTIFIER || p.cur.Type == token.MSG_KEYWORD ||
		p.cur.Type == token.LEN_KEYWORD || p.cur.Type == token._BEGIN ||
		p.cur.Type == token._START {
		op.Token = p.cur
		p.advance()
		return op, nil
	}

	// Number
	if p.cur.Type == token.NUMBER || p.cur.Type == token.HEX_LITERAL ||
		p.cur.Type == token.BIN_LITERAL {
		op.Token = p.cur
		p.advance()
		return op, nil
	}

	// $ - label  (length expression)
	if p.cur.Type == token.DOLLAR {
		op.Token = token.Token{Type: token.NUMBER, Literal: "0"}
		p.advance()
		if p.cur.Type == token.MINUS {
			p.advance() // skip -
			p.advance() // skip label name
		}
		return op, nil
	}

	return op, fmt.Errorf("unexpected operand token %q at line %d", p.cur.Literal, p.cur.Line)
}

// parseBracketExpr handles [a * z] shape syntax and [eax == name] aliases
func (p *Parser) parseBracketExpr() Node {
	line := p.cur.Line
	p.advance() // consume [

	// read first token inside brackets
	first := p.cur
	p.advance()

	// Alias: [eax == shortcutName]
	if p.cur.Type == token.ALIAS || p.cur.Type == token.EQUALS {
		p.advance()
		aliasName := p.cur.Literal
		p.advance()
		// optional = %rdmi; style
		if p.cur.Type == token.EQUALS || p.cur.Type == token.LPAREN {
			for p.cur.Type != token.RBRACKET && p.cur.Type != token.EOF {
				p.advance()
			}
		}
		if p.cur.Type == token.RBRACKET {
			p.advance()
		}
		regName := strings.ToLower(first.Literal)
		p.aliases[aliasName] = regName
		return AliasDecl{Register: regName, Name: aliasName, Line: line}
	}

	// Shape: [a * z] or [a * b] or [a * m]
	if p.cur.Type == token.STAR {
		p.advance()
		second := p.cur
		p.advance()
		if p.cur.Type == token.RBRACKET {
			p.advance()
		}
		shape := parseShapeKind(first.Literal, second.Literal)
		return Shape2D{Kind: shape, Width: shapeWidth(shape), Height: shapeHeight(shape), Line: line}
	}

	// eax[1] style shortcut (numeric register value)
	if p.cur.Type == token.NUMBER {
		val := p.cur.Literal
		p.advance()
		if p.cur.Type == token.RBRACKET {
			p.advance()
		}
		return Instruction{
			Instruction: "mov",
			Operands: []Operand{
				{Token: token.Token{Type: token.REGISTER, Literal: strings.ToLower(first.Literal)}},
				{Token: token.Token{Type: token.NUMBER, Literal: val}},
			},
			Line: line,
		}
	}

	if p.cur.Type == token.RBRACKET {
		p.advance()
	}
	return Error{Value: fmt.Sprintf("unknown bracket expression at line %d", line)}
}

func parseShapeKind(a, b string) string {
	a = strings.ToLower(a)
	b = strings.ToLower(b)
	switch {
	case a == "a" && b == "z":
		return "rect"
	case a == "a" && b == "b":
		return "rect"
	case a == "a" && b == "m":
		return "line"
	default:
		return "rect"
	}
}

func shapeWidth(kind string) int {
	switch kind {
	case "line":
		return 16
	default:
		return 8
	}
}

func shapeHeight(kind string) int {
	switch kind {
	case "line":
		return 1
	default:
		return 8
	}
}

func (p *Parser) parseRmpiBlock() Node {
	line := p.cur.Line
	p.advance()
	hexBytes := []string{}
	syscalls := []string{}

	for p.cur.Type != token.EOF {
		if p.cur.Type == token.SYSCALL {
			syscalls = append(syscalls, "syscall")
			p.advance()
			continue
		}
		if p.cur.Type == token.HEX_LITERAL {
			hexBytes = append(hexBytes, p.cur.Literal)
			p.advance()
			continue
		}
		if p.cur.Type == token.NEWLINE {
			p.advance()
			// end block on blank line
			if p.cur.Type == token.NEWLINE {
				break
			}
			continue
		}
		p.advance()
	}
	return RmpiExec{HexBytes: hexBytes, Syscalls: syscalls, Line: line}
}

func (p *Parser) parseLLtdBlock() Node {
	// LLtd [a * z] — the shape follows
	p.advance()
	if p.cur.Type == token.LBRACKET {
		node := p.parseBracketExpr()
		return node
	}
	return Section{Name: "lltd", Line: p.cur.Line}
}

func (p *Parser) parseCmdBlock() Node {
	line := p.cur.Line
	p.advance()
	// cmd[]== eax[1] ebx[4] ecx[len] edx[msg]
	ops := []string{}
	for p.cur.Type != token.NEWLINE && p.cur.Type != token.EOF {
		if p.cur.Type != token.COMMA {
			ops = append(ops, p.cur.Literal)
		}
		p.advance()
	}
	return CmdBlock{Operands: ops, Line: line}
}

func isDataDirective(t token.Type) bool {
	switch t {
	case token.DB, token.DW, token.DD, token.DQ:
		return true
	}
	return false
}

// Errors returns any parse errors collected
func (p *Parser) Errors() []string {
	return p.errors
}
