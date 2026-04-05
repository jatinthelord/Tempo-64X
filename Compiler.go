// Package compiler handles instruction encoding for Tempo.
package compiler

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"github.com/jatinthelord/tempo-64X/parser"
	"github.com/jatinthelord/tempo-64X/token"
)

// ---- Register helpers ----

var reg64Order = []string{
	"rax", "rcx", "rdx", "rbx", "rsp", "rbp", "rsi", "rdi",
	"r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15",
}
var reg32Order = []string{
	"eax", "ecx", "edx", "ebx", "esp", "ebp", "esi", "edi",
}

func regIndex64(r string) (int, bool) {
	for i, n := range reg64Order {
		if n == r {
			return i, true
		}
	}
	return -1, false
}

func regIndex32(r string) (int, bool) {
	for i, n := range reg32Order {
		if n == r {
			return i, true
		}
	}
	return -1, false
}

func isReg64(r string) bool { _, ok := regIndex64(r); return ok }
func isReg32(r string) bool { _, ok := regIndex32(r); return ok }
func isExtReg(r string) bool {
	ext := []string{"r8", "r9", "r10", "r11", "r12", "r13", "r14", "r15"}
	for _, e := range ext {
		if r == e {
			return true
		}
	}
	return false
}

// calcRM64 builds the ModRM byte for two 64-bit registers (mod=11)
func calcRM64(dest, src string) (byte, error) {
	di, ok1 := regIndex64(dest)
	si, ok2 := regIndex64(src)
	if !ok1 {
		return 0, fmt.Errorf("unknown register %q", dest)
	}
	if !ok2 {
		return 0, fmt.Errorf("unknown register %q", src)
	}
	return byte(0xc0 + (si%8)*8 + di%8), nil
}

// calcRM32 builds the ModRM byte for two 32-bit registers
func calcRM32(dest, src string) (byte, error) {
	di, ok1 := regIndex32(dest)
	si, ok2 := regIndex32(src)
	if !ok1 {
		return 0, fmt.Errorf("unknown register %q", dest)
	}
	if !ok2 {
		return 0, fmt.Errorf("unknown register %q", src)
	}
	return byte(0xc0 + si*8 + di), nil
}

// rexW returns 0x48 for basic 64-bit ops (REX.W)
// adds REX.R if src is extended, REX.B if dst is extended
func rexW(dst, src string) byte {
	b := byte(0x48)
	if isExtReg(src) {
		b |= 0x04 // REX.R
	}
	if isExtReg(dst) {
		b |= 0x01 // REX.B
	}
	return b
}

// parseImm64 parses a literal to int64 (handles hex, bin, dec)
func parseImm64(lit string) (int64, error) {
	lit = strings.TrimSpace(lit)
	if strings.HasPrefix(lit, "0x") || strings.HasPrefix(lit, "0X") {
		v, err := strconv.ParseInt(lit[2:], 16, 64)
		return v, err
	}
	if strings.HasPrefix(lit, "0b") || strings.HasPrefix(lit, "0B") {
		v, err := strconv.ParseInt(lit[2:], 2, 64)
		return v, err
	}
	return strconv.ParseInt(lit, 10, 64)
}

func imm32Bytes(v int64) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return b
}

func imm64Bytes(v int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(v))
	return b
}

// ---- Instruction encoders ----

func (c *Compiler) encodeMOV(i parser.Instruction) error {
	if len(i.Operands) < 2 {
		return fmt.Errorf("mov requires 2 operands at line %d", i.Line)
	}
	dst := i.Operands[0]
	src := i.Operands[1]

	// mov reg64, reg64
	if dst.Type == token.REGISTER && !dst.Indirection &&
		src.Type == token.REGISTER && !src.Indirection &&
		isReg64(dst.Literal) && isReg64(src.Literal) {
		rex := rexW(dst.Literal, src.Literal)
		rm, err := calcRM64(dst.Literal, src.Literal)
		if err != nil {
			return err
		}
		c.emit(rex, 0x89, rm)
		return nil
	}

	// mov reg32, reg32
	if dst.Type == token.REGISTER && !dst.Indirection &&
		src.Type == token.REGISTER && !src.Indirection &&
		isReg32(dst.Literal) && isReg32(src.Literal) {
		rm, err := calcRM32(dst.Literal, src.Literal)
		if err != nil {
			return err
		}
		c.emit(0x89, rm)
		return nil
	}

	// mov reg64, imm32
	if dst.Type == token.REGISTER && !dst.Indirection && isReg64(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL || src.Type == token.BIN_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		rex := rexW(dst.Literal, "")
		ri, _ := regIndex64(dst.Literal)
		c.emit(rex, 0xc7, byte(0xc0+ri%8))
		c.emitBytes(imm32Bytes(v))
		return nil
	}

	// mov reg32, imm32
	if dst.Type == token.REGISTER && !dst.Indirection && isReg32(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		ri, _ := regIndex32(dst.Literal)
		c.emit(byte(0xb8 + ri))
		c.emitBytes(imm32Bytes(v))
		return nil
	}

	// mov reg64, identifier (data label)
	if dst.Type == token.REGISTER && !dst.Indirection && isReg64(dst.Literal) &&
		(src.Type == token.IDENTIFIER || src.Type == token.MSG_KEYWORD || src.Type == token.LEN_KEYWORD) {
		name := src.Literal
		offset, ok := c.dataOffsets[name]
		if !ok {
			return fmt.Errorf("undefined data label %q at line %d", name, i.Line)
		}
		rex := rexW(dst.Literal, "")
		ri, _ := regIndex64(dst.Literal)
		c.emit(rex, 0xc7, byte(0xc0+ri%8))
		c.dataPatch(len(c.code), offset) // will be patched
		c.emitBytes([]byte{0, 0, 0, 0})
		return nil
	}

	// mov [reg64], imm  (indirection store)
	if dst.Type == token.REGISTER && dst.Indirection &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		ri, _ := regIndex64(dst.Literal)
		switch dst.Size {
		case parser.Size8:
			c.emit(0xc6, byte(ri))
			c.emit(byte(v))
		case parser.Size16:
			c.emit(0x66, 0xc7, byte(ri))
			b := make([]byte, 2)
			binary.LittleEndian.PutUint16(b, uint16(v))
			c.emitBytes(b)
		default: // 32/64
			c.emit(0xc7, byte(ri))
			c.emitBytes(imm32Bytes(v))
		}
		return nil
	}

	return fmt.Errorf("unsupported MOV form at line %d: %v <- %v", i.Line, dst.Literal, src.Literal)
}

func (c *Compiler) encodeADD(i parser.Instruction) error {
	if len(i.Operands) < 2 {
		return fmt.Errorf("add requires 2 operands")
	}
	dst, src := i.Operands[0], i.Operands[1]

	// add reg64, reg64
	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg64(dst.Literal) && isReg64(src.Literal) {
		rex := rexW(dst.Literal, src.Literal)
		rm, err := calcRM64(dst.Literal, src.Literal)
		if err != nil {
			return err
		}
		c.emit(rex, 0x01, rm)
		return nil
	}

	// add reg32, reg32
	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg32(dst.Literal) && isReg32(src.Literal) {
		rm, err := calcRM32(dst.Literal, src.Literal)
		if err != nil {
			return err
		}
		c.emit(0x01, rm)
		return nil
	}

	// add reg64, imm32
	if dst.Type == token.REGISTER && isReg64(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		rex := rexW(dst.Literal, "")
		switch dst.Literal {
		case "rax":
			c.emit(rex, 0x05)
		default:
			ri, _ := regIndex64(dst.Literal)
			c.emit(rex, 0x81, byte(0xc0+ri%8))
		}
		c.emitBytes(imm32Bytes(v))
		return nil
	}

	// add reg32, imm32
	if dst.Type == token.REGISTER && isReg32(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		ri, _ := regIndex32(dst.Literal)
		c.emit(0x81, byte(0xc0+ri))
		c.emitBytes(imm32Bytes(v))
		return nil
	}

	return fmt.Errorf("unsupported ADD form at line %d", i.Line)
}

func (c *Compiler) encodeSUB(i parser.Instruction) error {
	if len(i.Operands) < 2 {
		return fmt.Errorf("sub requires 2 operands")
	}
	dst, src := i.Operands[0], i.Operands[1]

	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg64(dst.Literal) && isReg64(src.Literal) {
		rex := rexW(dst.Literal, src.Literal)
		rm, err := calcRM64(dst.Literal, src.Literal)
		if err != nil {
			return err
		}
		c.emit(rex, 0x29, rm)
		return nil
	}
	if dst.Type == token.REGISTER && isReg64(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		rex := rexW(dst.Literal, "")
		switch dst.Literal {
		case "rax":
			c.emit(rex, 0x2d)
		default:
			ri, _ := regIndex64(dst.Literal)
			c.emit(rex, 0x81, byte(0xe8+ri%8))
		}
		c.emitBytes(imm32Bytes(v))
		return nil
	}
	if dst.Type == token.REGISTER && isReg32(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		ri, _ := regIndex32(dst.Literal)
		c.emit(0x81, byte(0xe8+ri))
		c.emitBytes(imm32Bytes(v))
		return nil
	}
	return fmt.Errorf("unsupported SUB form at line %d", i.Line)
}

func (c *Compiler) encodeXOR(i parser.Instruction) error {
	if len(i.Operands) < 2 {
		return fmt.Errorf("xor requires 2 operands")
	}
	dst, src := i.Operands[0], i.Operands[1]
	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg64(dst.Literal) && isReg64(src.Literal) {
		rex := rexW(dst.Literal, src.Literal)
		rm, err := calcRM64(dst.Literal, src.Literal)
		if err != nil {
			return err
		}
		c.emit(rex, 0x31, rm)
		return nil
	}
	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg32(dst.Literal) && isReg32(src.Literal) {
		rm, err := calcRM32(dst.Literal, src.Literal)
		if err != nil {
			return err
		}
		c.emit(0x31, rm)
		return nil
	}
	return fmt.Errorf("unsupported XOR form at line %d", i.Line)
}

func (c *Compiler) encodeAND(i parser.Instruction) error {
	if len(i.Operands) < 2 {
		return fmt.Errorf("and requires 2 operands")
	}
	dst, src := i.Operands[0], i.Operands[1]
	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg64(dst.Literal) && isReg64(src.Literal) {
		rex := rexW(dst.Literal, src.Literal)
		rm, _ := calcRM64(dst.Literal, src.Literal)
		c.emit(rex, 0x21, rm)
		return nil
	}
	if dst.Type == token.REGISTER && isReg64(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		rex := rexW(dst.Literal, "")
		ri, _ := regIndex64(dst.Literal)
		c.emit(rex, 0x81, byte(0xe0+ri%8))
		c.emitBytes(imm32Bytes(v))
		return nil
	}
	return fmt.Errorf("unsupported AND form at line %d", i.Line)
}

func (c *Compiler) encodeOR(i parser.Instruction) error {
	if len(i.Operands) < 2 {
		return fmt.Errorf("or requires 2 operands")
	}
	dst, src := i.Operands[0], i.Operands[1]
	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg64(dst.Literal) && isReg64(src.Literal) {
		rex := rexW(dst.Literal, src.Literal)
		rm, _ := calcRM64(dst.Literal, src.Literal)
		c.emit(rex, 0x09, rm)
		return nil
	}
	return fmt.Errorf("unsupported OR form at line %d", i.Line)
}

func (c *Compiler) encodeINC(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("inc requires 1 operand")
	}
	op := i.Operands[0]
	if op.Type == token.REGISTER && !op.Indirection && isReg64(op.Literal) {
		rex := rexW(op.Literal, "")
		ri, _ := regIndex64(op.Literal)
		c.emit(rex, 0xff, byte(0xc0+ri%8))
		return nil
	}
	if op.Type == token.REGISTER && !op.Indirection && isReg32(op.Literal) {
		ri, _ := regIndex32(op.Literal)
		c.emit(0xff, byte(0xc0+ri))
		return nil
	}
	return fmt.Errorf("unsupported INC form at line %d", i.Line)
}

func (c *Compiler) encodeDEC(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("dec requires 1 operand")
	}
	op := i.Operands[0]
	if op.Type == token.REGISTER && !op.Indirection && isReg64(op.Literal) {
		rex := rexW(op.Literal, "")
		ri, _ := regIndex64(op.Literal)
		c.emit(rex, 0xff, byte(0xc8+ri%8))
		return nil
	}
	if op.Type == token.REGISTER && !op.Indirection && isReg32(op.Literal) {
		ri, _ := regIndex32(op.Literal)
		c.emit(0xff, byte(0xc8+ri))
		return nil
	}
	return fmt.Errorf("unsupported DEC form at line %d", i.Line)
}

func (c *Compiler) encodePUSH(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("push requires 1 operand")
	}
	op := i.Operands[0]

	pushRegs64 := map[string]byte{
		"rax": 0x50, "rcx": 0x51, "rdx": 0x52, "rbx": 0x53,
		"rsp": 0x54, "rbp": 0x55, "rsi": 0x56, "rdi": 0x57,
	}
	pushRegsExt := map[string][]byte{
		"r8": {0x41, 0x50}, "r9": {0x41, 0x51}, "r10": {0x41, 0x52},
		"r11": {0x41, 0x53}, "r12": {0x41, 0x54}, "r13": {0x41, 0x55},
		"r14": {0x41, 0x56}, "r15": {0x41, 0x57},
	}

	if op.Type == token.REGISTER {
		if b, ok := pushRegs64[op.Literal]; ok {
			c.emit(b)
			return nil
		}
		if bs, ok := pushRegsExt[op.Literal]; ok {
			c.emitBytes(bs)
			return nil
		}
	}
	if op.Type == token.NUMBER || op.Type == token.HEX_LITERAL {
		v, err := parseImm64(op.Literal)
		if err != nil {
			return err
		}
		c.emit(0x68)
		c.emitBytes(imm32Bytes(v))
		return nil
	}
	if op.Type == token.IDENTIFIER {
		c.emit(0x68)
		c.labelTarget(len(c.code), op.Literal)
		c.emitBytes([]byte{0, 0, 0, 0})
		return nil
	}
	return fmt.Errorf("unsupported PUSH form at line %d", i.Line)
}

func (c *Compiler) encodePOP(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("pop requires 1 operand")
	}
	op := i.Operands[0]
	popRegs64 := map[string]byte{
		"rax": 0x58, "rbx": 0x5b, "rcx": 0x59, "rdx": 0x5a,
		"rbp": 0x5d, "rsp": 0x5c, "rsi": 0x5e, "rdi": 0x5f,
	}
	popRegsExt := map[string][]byte{
		"r8": {0x41, 0x58}, "r9": {0x41, 0x59}, "r10": {0x41, 0x5a},
		"r11": {0x41, 0x5b}, "r12": {0x41, 0x5c}, "r13": {0x41, 0x5d},
		"r14": {0x41, 0x5e}, "r15": {0x41, 0x5f},
	}
	if op.Type == token.REGISTER {
		if b, ok := popRegs64[op.Literal]; ok {
			c.emit(b)
			return nil
		}
		if bs, ok := popRegsExt[op.Literal]; ok {
			c.emitBytes(bs)
			return nil
		}
	}
	return fmt.Errorf("unsupported POP form at line %d", i.Line)
}

func (c *Compiler) encodeCALL(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("call requires a label operand")
	}
	op := i.Operands[0]
	if op.Type == token.IDENTIFIER || op.Type == token._BEGIN || op.Type == token._START {
		c.emit(0xe8)
		c.callTarget(len(c.code), op.Literal)
		c.emitBytes([]byte{0, 0, 0, 0})
		return nil
	}
	// call register
	if op.Type == token.REGISTER && isReg64(op.Literal) {
		ri, _ := regIndex64(op.Literal)
		c.emit(0xff, byte(0xd0+ri%8))
		return nil
	}
	return fmt.Errorf("unsupported CALL form at line %d", i.Line)
}

func (c *Compiler) encodeJMP(instr string, i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("jump requires a label operand")
	}
	op := i.Operands[0]
	var opcode byte
	switch strings.ToLower(instr) {
	case "jmp", "jns": // jns is Tempo's jmp
		opcode = 0xeb
	case "je", "jz", "jeq":
		opcode = 0x74
	case "jne", "jnz":
		opcode = 0x75
	default:
		opcode = 0xeb
	}
	if op.Type == token.IDENTIFIER || op.Type == token._BEGIN || op.Type == token._START {
		c.emit(opcode)
		c.jmpTarget(len(c.code), op.Literal)
		c.emit(0x00)
		return nil
	}
	return fmt.Errorf("unsupported JMP form at line %d", i.Line)
}

func (c *Compiler) encodeCMP(i parser.Instruction) error {
	if len(i.Operands) < 2 {
		return fmt.Errorf("cmp/clp requires 2 operands")
	}
	dst, src := i.Operands[0], i.Operands[1]
	if dst.Type == token.REGISTER && src.Type == token.REGISTER &&
		isReg64(dst.Literal) && isReg64(src.Literal) {
		rex := rexW(dst.Literal, src.Literal)
		rm, _ := calcRM64(dst.Literal, src.Literal)
		c.emit(rex, 0x39, rm)
		return nil
	}
	if dst.Type == token.REGISTER && isReg64(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		rex := rexW(dst.Literal, "")
		ri, _ := regIndex64(dst.Literal)
		if dst.Literal == "rax" {
			c.emit(rex, 0x3d)
		} else {
			c.emit(rex, 0x81, byte(0xf8+ri%8))
		}
		c.emitBytes(imm32Bytes(v))
		return nil
	}
	if dst.Type == token.REGISTER && isReg32(dst.Literal) &&
		(src.Type == token.NUMBER || src.Type == token.HEX_LITERAL) {
		v, err := parseImm64(src.Literal)
		if err != nil {
			return err
		}
		ri, _ := regIndex32(dst.Literal)
		c.emit(0x81, byte(0xf8+ri))
		c.emitBytes(imm32Bytes(v))
		return nil
	}
	return fmt.Errorf("unsupported CMP/CLP form at line %d", i.Line)
}

func (c *Compiler) encodeINT(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("int requires 1 operand")
	}
	v, err := parseImm64(i.Operands[0].Literal)
	if err != nil {
		return err
	}
	c.emit(0xcd, byte(v))
	return nil
}

func (c *Compiler) encodeMUL(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("mul requires 1 operand")
	}
	op := i.Operands[0]
	if op.Type == token.REGISTER && isReg64(op.Literal) {
		rex := rexW(op.Literal, "")
		ri, _ := regIndex64(op.Literal)
		c.emit(rex, 0xf7, byte(0xe0+ri%8))
		return nil
	}
	return fmt.Errorf("unsupported MUL form at line %d", i.Line)
}

func (c *Compiler) encodeDIV(i parser.Instruction) error {
	if len(i.Operands) < 1 {
		return fmt.Errorf("div requires 1 operand")
	}
	op := i.Operands[0]
	if op.Type == token.REGISTER && isReg64(op.Literal) {
		rex := rexW(op.Literal, "")
		ri, _ := regIndex64(op.Literal)
		c.emit(rex, 0xf7, byte(0xf0+ri%8))
		return nil
	}
	return fmt.Errorf("unsupported DIV form at line %d", i.Line)
}
