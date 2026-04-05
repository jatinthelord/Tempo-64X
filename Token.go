// Package token defines all token types for the Tempo language.
package token

// Type is the type of a token
type Type string

const (
	// Literals
	ILLEGAL    Type = "ILLEGAL"
	EOF        Type = "EOF"
	IDENTIFIER Type = "IDENTIFIER"
	NUMBER     Type = "NUMBER"
	STRING     Type = "STRING"

	// Register types
	REGISTER Type = "REGISTER"

	// Sections
	SECTION_DATA Type = "SECTION_DATA"
	SECTION_TEXT Type = "SECTION_TEXT"

	// Tempo-specific keywords
	JE     Type = "JE"     // object marker
	NN     Type = "NN"     // load opcodes / no-op padding
	QR     Type = "QR"     // end-of-section marker
	RMPI   Type = "RMPI"   // execute/convert files
	LLTD   Type = "LLTD"   // 2D object begin
	LLMP   Type = "LLMP"   // LLmp ROM cracker (restricted)
	CMD    Type = "CMD"    // shortcut command block
	_BEGIN Type = "_BEGIN" // entry point label
	_START Type = "_START" // alternate entry point
	_GLOBAL Type = "_GLOBAL" // global symbol

	// Instructions
	MOV      Type = "MOV"
	ADD      Type = "ADD"
	SUB      Type = "SUB"
	MUL      Type = "MUL"
	DIV      Type = "DIV"
	XOR      Type = "XOR"
	AND      Type = "AND"
	OR       Type = "OR"
	NOT      Type = "NOT"
	INC      Type = "INC"
	DEC      Type = "DEC"
	PUSH     Type = "PUSH"
	POP      Type = "POP"
	CALL     Type = "CALL"
	RET      Type = "RET"
	NOP      Type = "NOP"
	INT      Type = "INT"
	SYSCALL  Type = "SYSCALL"
	HLT      Type = "HLT"

	// Tempo jump instructions
	JNS Type = "JNS" // Tempo jump (like jmp)
	JMP Type = "JMP"
	JNZ Type = "JNZ"
	JNE Type = "JNE"
	JZ  Type = "JZ"
	JEQ Type = "JEQ"

	// Tempo compare
	CLP Type = "CLP" // Tempo compare (like cmp)
	CMP Type = "CMP"

	// Flags
	CLF Type = "CLF"
	STF Type = "STF"

	// Data directives
	DB   Type = "DB"
	DW   Type = "DW"
	DD   Type = "DD"
	DQ   Type = "DQ"
	EQU  Type = "EQU"
	BYTE Type = "BYTE"

	// Delimiters
	COMMA     Type = "COMMA"
	COLON     Type = "COLON"
	SEMICOLON Type = "SEMICOLON"
	LBRACKET  Type = "LBRACKET"
	RBRACKET  Type = "RBRACKET"
	LPAREN    Type = "LPAREN"
	RPAREN    Type = "RPAREN"
	STAR      Type = "STAR"
	DOLLAR    Type = "DOLLAR"
	PERCENT   Type = "PERCENT"
	NEWLINE   Type = "NEWLINE"
	EQUALS    Type = "EQUALS"
	MINUS     Type = "MINUS"
	DOT       Type = "DOT"

	// Size qualifiers
	SIZE_BYTE  Type = "SIZE_BYTE"
	SIZE_WORD  Type = "SIZE_WORD"
	SIZE_DWORD Type = "SIZE_DWORD"
	SIZE_QWORD Type = "SIZE_QWORD"
	SIZE_PTR   Type = "SIZE_PTR"

	// Hex literal
	HEX_LITERAL Type = "HEX_LITERAL"

	// Binary literal
	BIN_LITERAL Type = "BIN_LITERAL"

	// Alias shortcut
	ALIAS Type = "ALIAS"

	// Special
	BYTES_KEYWORD Type = "BYTES_KEYWORD"
	MSG_KEYWORD   Type = "MSG_KEYWORD"
	LEN_KEYWORD   Type = "LEN_KEYWORD"
	INT0X80       Type = "INT0X80"
)

// Token holds a single token
type Token struct {
	Type    Type
	Literal string
	Line    int
	Col     int
}

// keywords maps identifiers to their token types
var keywords = map[string]Type{
	// Sections
	"Section": IDENTIFIER, // handled specially

	// Entry points
	"_begin":  _BEGIN,
	"_Begin":  _BEGIN,
	"_START":  _START,
	"_start":  _START,
	"_global": _GLOBAL,
	"_Global": _GLOBAL,

	// Tempo special tokens
	"je":   JE,
	"nn":   NN,
	"qr":   QR,
	"LLtd": LLTD,
	"lltd": LLTD,
	"LLmp": LLMP,
	"llmp": LLMP,
	"cmd":  CMD,

	// Instructions (case-insensitive handled in lexer)
	"mov":     MOV,
	"MOV":     MOV,
	"add":     ADD,
	"ADD":     ADD,
	"sub":     SUB,
	"SUB":     SUB,
	"mul":     MUL,
	"MUL":     MUL,
	"div":     DIV,
	"DIV":     DIV,
	"xor":     XOR,
	"XOR":     XOR,
	"and":     AND,
	"AND":     AND,
	"or":      OR,
	"OR":      OR,
	"not":     NOT,
	"NOT":     NOT,
	"inc":     INC,
	"INC":     INC,
	"dec":     DEC,
	"DEC":     DEC,
	"push":    PUSH,
	"PUSH":    PUSH,
	"pop":     POP,
	"POP":     POP,
	"call":    CALL,
	"CALL":    CALL,
	"ret":     RET,
	"RET":     RET,
	"nop":     NOP,
	"NOP":     NOP,
	"int":     INT,
	"INT":     INT,
	"syscall": SYSCALL,
	"SYSCALL": SYSCALL,
	"hlt":     HLT,
	"HLT":     HLT,

	// Tempo jump / compare
	"jns": JNS,
	"JNS": JNS,
	"jmp": JMP,
	"JMP": JMP,
	"jnz": JNZ,
	"JNZ": JNZ,
	"jne": JNE,
	"JNE": JNE,
	"jz":  JZ,
	"JZ":  JZ,
	"je":  JEQ,
	"clp": CLP,
	"CLP": CLP,
	"cmp": CMP,
	"CMP": CMP,

	// Data
	"db":    DB,
	"DB":    DB,
	"dw":    DW,
	"DW":    DW,
	"dd":    DD,
	"DD":    DD,
	"dq":    DQ,
	"DQ":    DQ,
	"equ":   EQU,
	"EQU":   EQU,
	"byte":  BYTE,
	"BYTE":  BYTE,
	"msg":   MSG_KEYWORD,
	"len":   LEN_KEYWORD,
	".bytes": BYTES_KEYWORD,

	// Size qualifiers
	"byte ptr":  SIZE_PTR,
	"word ptr":  SIZE_PTR,
	"dword ptr": SIZE_PTR,
	"qword ptr": SIZE_PTR,
	"ptr":       SIZE_PTR,
}

// LookupIdentifier checks if ident is a keyword
func LookupIdentifier(ident string) Type {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENTIFIER
}

// IsRegister returns true if the string is a known register
func IsRegister(s string) bool {
	registers := map[string]bool{
		// 64-bit
		"rax": true, "rbx": true, "rcx": true, "rdx": true,
		"rsi": true, "rdi": true, "rsp": true, "rbp": true,
		"r8": true, "r9": true, "r10": true, "r11": true,
		"r12": true, "r13": true, "r14": true, "r15": true,
		// 32-bit
		"eax": true, "ebx": true, "ecx": true, "edx": true,
		"esi": true, "edi": true, "esp": true, "ebp": true,
		// 16-bit
		"ax": true, "bx": true, "cx": true, "dx": true,
		// 8-bit
		"al": true, "ah": true, "bl": true, "bh": true,
		"cl": true, "ch": true, "dl": true, "dh": true,
		// special
		"rip": true, "rflags": true,
		// Tempo aliases built-in
		"%rdi%": true, "%rdmi%": true, "%rmpi%": true,
	}
	return registers[s]
}
