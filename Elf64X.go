// Package elf writes minimal Linux ELF64 executables.
package elf

import (
	"encoding/binary"
	"os"
)

const (
	elfBase    = 0x400000
	elfHdrSize = 0x40
	progHdrSz  = 0x38
	numPHdr    = 2
)

// Writer produces ELF64 Linux executables
type Writer struct{}

// New creates a new ELF writer
func New() *Writer { return &Writer{} }

// WriteContent writes a complete ELF64 binary to path.
// code is the .text section, data is the .data section.
func (w *Writer) WriteContent(path string, code []byte, data []byte) error {
	textOffset := uint64(elfHdrSize + numPHdr*progHdrSz)
	textAddr := uint64(elfBase) + textOffset
	textSize := uint64(len(code))

	dataOffset := textOffset + textSize
	// align data to 16 bytes
	if dataOffset%16 != 0 {
		pad := 16 - dataOffset%16
		dataOffset += pad
	}
	dataAddr := uint64(elfBase) + dataOffset
	dataSize := uint64(len(data))

	totalSize := dataOffset + dataSize

	buf := make([]byte, totalSize)

	// ---- ELF Header ----
	copy(buf[0:], []byte{0x7f, 'E', 'L', 'F'}) // magic
	buf[4] = 2                                   // 64-bit
	buf[5] = 1                                   // little endian
	buf[6] = 1                                   // ELF version
	buf[7] = 0                                   // OS/ABI = SysV
	// e_type = ET_EXEC
	binary.LittleEndian.PutUint16(buf[16:], 2)
	// e_machine = x86-64
	binary.LittleEndian.PutUint16(buf[18:], 0x3e)
	// e_version
	binary.LittleEndian.PutUint32(buf[20:], 1)
	// e_entry
	binary.LittleEndian.PutUint64(buf[24:], textAddr)
	// e_phoff = 0x40
	binary.LittleEndian.PutUint64(buf[32:], elfHdrSize)
	// e_shoff = 0
	binary.LittleEndian.PutUint64(buf[40:], 0)
	// e_flags
	binary.LittleEndian.PutUint32(buf[48:], 0)
	// e_ehsize
	binary.LittleEndian.PutUint16(buf[52:], elfHdrSize)
	// e_phentsize
	binary.LittleEndian.PutUint16(buf[54:], progHdrSz)
	// e_phnum
	binary.LittleEndian.PutUint16(buf[56:], numPHdr)
	// e_shentsize
	binary.LittleEndian.PutUint16(buf[58:], 0x40)
	// e_shnum
	binary.LittleEndian.PutUint16(buf[60:], 0)
	// e_shstrndx
	binary.LittleEndian.PutUint16(buf[62:], 0)

	// ---- Program Header 0: LOAD (.text) ----
	ph0 := elfHdrSize
	binary.LittleEndian.PutUint32(buf[ph0:], 1)          // PT_LOAD
	binary.LittleEndian.PutUint32(buf[ph0+4:], 5)        // PF_R | PF_X
	binary.LittleEndian.PutUint64(buf[ph0+8:], textOffset)
	binary.LittleEndian.PutUint64(buf[ph0+16:], textAddr)
	binary.LittleEndian.PutUint64(buf[ph0+24:], textAddr)
	binary.LittleEndian.PutUint64(buf[ph0+32:], textSize)
	binary.LittleEndian.PutUint64(buf[ph0+40:], textSize)
	binary.LittleEndian.PutUint64(buf[ph0+48:], 0x200000)

	// ---- Program Header 1: LOAD (.data) ----
	ph1 := elfHdrSize + progHdrSz
	binary.LittleEndian.PutUint32(buf[ph1:], 1)          // PT_LOAD
	binary.LittleEndian.PutUint32(buf[ph1+4:], 6)        // PF_R | PF_W
	binary.LittleEndian.PutUint64(buf[ph1+8:], dataOffset)
	binary.LittleEndian.PutUint64(buf[ph1+16:], dataAddr)
	binary.LittleEndian.PutUint64(buf[ph1+24:], dataAddr)
	binary.LittleEndian.PutUint64(buf[ph1+32:], dataSize)
	binary.LittleEndian.PutUint64(buf[ph1+40:], dataSize)
	binary.LittleEndian.PutUint64(buf[ph1+48:], 0x200000)

	// ---- Copy code and data ----
	copy(buf[textOffset:], code)
	copy(buf[dataOffset:], data)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(buf)
	return err
}
