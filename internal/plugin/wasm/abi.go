// Package wasm implements the WASM plugin tier (ADR-0020, ADR-0045, ADR-0056).
//
// It wraps wazero with the demitable host ABI: one exported entry point
// `demitable_call(ptr, len) -> ptr` and the host functions, each granted only
// when the plugin's manifest declares the matching capability.
package wasm

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Host function names in the "env" module namespace. A guest imports them
// from "env". These names are part of api: 1.
const (
	HostModule = "env"

	FnLog         = "host_log"
	FnVaultRead   = "host_vault_read"
	FnVaultWrite  = "host_vault_write"
	FnVaultQuery  = "host_vault_query"
	FnVaultAssets = "host_vault_assets"
	FnHTTPFetch   = "host_http_fetch"
	FnNow         = "host_now"
	FnRandom      = "host_random"
)

// AllHostFunctions is the ordered list of host functions the host offers.
// The golden ABI module imports every one of them.
var AllHostFunctions = []string{
	FnLog,
	FnVaultRead,
	FnVaultWrite,
	FnVaultQuery,
	FnVaultAssets,
	FnHTTPFetch,
	FnNow,
	FnRandom,
}

// --- tiny WASM encoder ---------------------------------------------------

func appendVarUint(buf []byte, v uint32) []byte {
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

func section(id byte, content []byte) []byte {
	out := []byte{id}
	out = appendVarUint(out, uint32(len(content)))
	out = append(out, content...)
	return out
}

// encodeTypeFunc builds a functype: 0x60 params results.
func encodeTypeFunc(params, results []byte) []byte {
	out := []byte{0x60}
	out = appendVarUint(out, uint32(len(params)))
	out = append(out, params...)
	out = appendVarUint(out, uint32(len(results)))
	out = append(out, results...)
	return out
}

const (
	valI32 = 0x7f
	valI64 = 0x7e
)

// GoldenABIModule returns a WASM module that imports every host function the
// host offers and exports `demitable_call`. It is the golden ABI freeze test's
// input: if any host function signature moves, instantiation fails.
func GoldenABIModule() []byte {
	// types: one type for (i32,i32)->i32 (used by most + demitable_call),
	// and one for ()->i64 (host_now).
	t0 := encodeTypeFunc([]byte{valI32, valI32}, []byte{valI32}) // type 0
	t1 := encodeTypeFunc(nil, []byte{valI64})                    // type 1

	typeSec := []byte{}
	typeSec = appendVarUint(typeSec, 2) // count
	typeSec = append(typeSec, t0...)
	typeSec = append(typeSec, t1...)

	// imports: 7 (i32,i32)->i32 + host_now ()->i64
	importNames := []string{FnLog, FnVaultRead, FnVaultWrite, FnVaultQuery, FnVaultAssets, FnHTTPFetch, FnRandom}
	importSec := []byte{}
	importSec = appendVarUint(importSec, uint32(len(importNames)+1))
	for _, name := range importNames {
		// module name "env"
		importSec = append(importSec, encodeString(HostModule)...)
		importSec = append(importSec, encodeString(name)...)
		importSec = append(importSec, 0x00)     // func import
		importSec = appendVarUint(importSec, 0) // type index 0
	}
	// host_now
	importSec = append(importSec, encodeString(HostModule)...)
	importSec = append(importSec, encodeString(FnNow)...)
	importSec = append(importSec, 0x00)
	importSec = appendVarUint(importSec, 1) // type index 1

	// function section: one defined function (demitable_call) type 0
	funcSec := []byte{}
	funcSec = appendVarUint(funcSec, 1)
	funcSec = appendVarUint(funcSec, 0)

	// memory section: min 2 pages
	memSec := []byte{}
	memSec = appendVarUint(memSec, 1) // count
	memSec = append(memSec, 0x00)     // limits with min only
	memSec = appendVarUint(memSec, 2)

	// export section: demitable_call func index = numImports; memory "memory" index 0
	numImports := uint32(len(importNames) + 1)
	exportSec := []byte{}
	exportSec = appendVarUint(exportSec, 2)
	// func export
	exportSec = append(exportSec, encodeString("demitable_call")...)
	exportSec = append(exportSec, 0x00)
	exportSec = appendVarUint(exportSec, numImports)
	// memory export
	exportSec = append(exportSec, encodeString("memory")...)
	exportSec = append(exportSec, 0x02)
	exportSec = appendVarUint(exportSec, 0)

	// code section: demitable_call body: i32.const 0; end
	codeBody := []byte{0x00}                // local count
	codeBody = append(codeBody, 0x41, 0x00) // i32.const 0
	codeBody = append(codeBody, 0x0b)       // end
	codeSec := []byte{}
	codeSec = appendVarUint(codeSec, 1) // count
	// each code entry: size + body
	sz := make([]byte, 0, 4)
	sz = appendVarUint(sz, uint32(len(codeBody)))
	codeSec = append(codeSec, sz...)
	codeSec = append(codeSec, codeBody...)

	// assemble
	out := []byte{0x00, 'a', 's', 'm', 0x01, 0x00, 0x00, 0x00}
	out = append(out, section(1, typeSec)...)
	out = append(out, section(2, importSec)...)
	out = append(out, section(3, funcSec)...)
	out = append(out, section(5, memSec)...)
	out = append(out, section(7, exportSec)...)
	out = append(out, section(10, codeSec)...)
	return out
}

func encodeString(s string) []byte {
	out := []byte{}
	out = appendVarUint(out, uint32(len(s)))
	out = append(out, []byte(s)...)
	return out
}

// ReadModule returns a module that, when demitable_call is invoked, calls
// host_vault_read with a request for the note uid "n1" and returns the
// host's response pointer. Used by tests proving a granted read works.
func ReadModule() []byte {
	req := []byte(`{"uid":"n1"}`)
	// type 0: (i32,i32)->i32
	t0 := encodeTypeFunc([]byte{valI32, valI32}, []byte{valI32})
	typeSec := []byte{}
	typeSec = appendVarUint(typeSec, 1)
	typeSec = append(typeSec, t0...)

	// imports: host_vault_read only (index 0)
	importSec := []byte{}
	importSec = appendVarUint(importSec, 1)
	importSec = append(importSec, encodeString(HostModule)...)
	importSec = append(importSec, encodeString(FnVaultRead)...)
	importSec = append(importSec, 0x00)
	importSec = appendVarUint(importSec, 0)

	// function section
	funcSec := []byte{}
	funcSec = appendVarUint(funcSec, 1)
	funcSec = appendVarUint(funcSec, 0)

	// memory
	memSec := []byte{}
	memSec = appendVarUint(memSec, 1)
	memSec = append(memSec, 0x00)
	memSec = appendVarUint(memSec, 2)

	// data section: store req at offset 1024
	dataSec := []byte{}
	dataSec = appendVarUint(dataSec, 1)
	// segment: memory 0, offset expr i32.const 1024 end, then bytes
	seg := []byte{}
	seg = appendVarUint(seg, 0) // memory index
	seg = append(seg, 0x41)     // i32.const
	seg = appendVarUint(seg, 1024)
	seg = append(seg, 0x0b) // end
	seg = appendVarUint(seg, uint32(len(req)))
	seg = append(seg, req...)
	dataSec = append(dataSec, seg...)

	// export
	exportSec := []byte{}
	exportSec = appendVarUint(exportSec, 2)
	exportSec = append(exportSec, encodeString("demitable_call")...)
	exportSec = append(exportSec, 0x00)
	exportSec = appendVarUint(exportSec, 1) // defined func index = numImports =1
	exportSec = append(exportSec, encodeString("memory")...)
	exportSec = append(exportSec, 0x02)
	exportSec = appendVarUint(exportSec, 0)

	// code: i32.const 1024; i32.const len; call 0; end
	codeBody := []byte{0x00}
	codeBody = append(codeBody, 0x41)
	codeBody = appendVarUint(codeBody, 1024)
	codeBody = append(codeBody, 0x41)
	codeBody = appendVarUint(codeBody, uint32(len(req)))
	codeBody = append(codeBody, 0x10, 0x00) // call func 0
	codeBody = append(codeBody, 0x0b)
	codeSec := []byte{}
	codeSec = appendVarUint(codeSec, 1)
	sz := []byte{}
	sz = appendVarUint(sz, uint32(len(codeBody)))
	codeSec = append(codeSec, sz...)
	codeSec = append(codeSec, codeBody...)

	out := []byte{0x00, 'a', 's', 'm', 0x01, 0x00, 0x00, 0x00}
	out = append(out, section(1, typeSec)...)
	out = append(out, section(2, importSec)...)
	out = append(out, section(3, funcSec)...)
	out = append(out, section(5, memSec)...)
	out = append(out, section(7, exportSec)...)
	out = append(out, section(10, codeSec)...)
	out = append(out, section(11, dataSec)...)
	return out
}

// WriteAttemptModule returns a module that calls host_vault_write (undeclared)
// and returns the host's response pointer.
func WriteAttemptModule() []byte {
	req := []byte(`{"uid":"n1","body":"x"}`)
	t0 := encodeTypeFunc([]byte{valI32, valI32}, []byte{valI32})
	typeSec := []byte{}
	typeSec = appendVarUint(typeSec, 1)
	typeSec = append(typeSec, t0...)

	importSec := []byte{}
	importSec = appendVarUint(importSec, 1)
	importSec = append(importSec, encodeString(HostModule)...)
	importSec = append(importSec, encodeString(FnVaultWrite)...)
	importSec = append(importSec, 0x00)
	importSec = appendVarUint(importSec, 0)

	funcSec := []byte{}
	funcSec = appendVarUint(funcSec, 1)
	funcSec = appendVarUint(funcSec, 0)

	memSec := []byte{}
	memSec = appendVarUint(memSec, 1)
	memSec = append(memSec, 0x00)
	memSec = appendVarUint(memSec, 2)

	dataSec := []byte{}
	dataSec = appendVarUint(dataSec, 1)
	seg := []byte{}
	seg = appendVarUint(seg, 0)
	seg = append(seg, 0x41)
	seg = appendVarUint(seg, 1024)
	seg = append(seg, 0x0b)
	seg = appendVarUint(seg, uint32(len(req)))
	seg = append(seg, req...)
	dataSec = append(dataSec, seg...)

	exportSec := []byte{}
	exportSec = appendVarUint(exportSec, 2)
	exportSec = append(exportSec, encodeString("demitable_call")...)
	exportSec = append(exportSec, 0x00)
	exportSec = appendVarUint(exportSec, 1)
	exportSec = append(exportSec, encodeString("memory")...)
	exportSec = append(exportSec, 0x02)
	exportSec = appendVarUint(exportSec, 0)

	codeBody := []byte{0x00}
	codeBody = append(codeBody, 0x41)
	codeBody = appendVarUint(codeBody, 1024)
	codeBody = append(codeBody, 0x41)
	codeBody = appendVarUint(codeBody, uint32(len(req)))
	codeBody = append(codeBody, 0x10, 0x00)
	codeBody = append(codeBody, 0x0b)
	codeSec := []byte{}
	codeSec = appendVarUint(codeSec, 1)
	sz := []byte{}
	sz = appendVarUint(sz, uint32(len(codeBody)))
	codeSec = append(codeSec, sz...)
	codeSec = append(codeSec, codeBody...)

	out := []byte{0x00, 'a', 's', 'm', 0x01, 0x00, 0x00, 0x00}
	out = append(out, section(1, typeSec)...)
	out = append(out, section(2, importSec)...)
	out = append(out, section(3, funcSec)...)
	out = append(out, section(5, memSec)...)
	out = append(out, section(7, exportSec)...)
	out = append(out, section(10, codeSec)...)
	out = append(out, section(11, dataSec)...)
	return out
}

func bytesIndexByte(b []byte, c byte) int {
	for i, x := range b {
		if x == c {
			return i
		}
	}
	return -1
}

// unused guard
var _ = bytesIndexByte
var _ = fmt.Errorf
var _ = bytes.NewBuffer
var _ = binary.LittleEndian
