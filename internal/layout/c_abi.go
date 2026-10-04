package layout

import "fmt"

// CFloatFormat is the physical format the active C ABI selects for a C
// floating type.
type CFloatFormat string

const (
	CFloatBinary32         CFloatFormat = "ieee-binary32"
	CFloatBinary64         CFloatFormat = "ieee-binary64"
	CFloatX87Extended80    CFloatFormat = "x87-extended80"
	CFloatBinary128        CFloatFormat = "ieee-binary128"
	cFloatFormatUnresolved CFloatFormat = ""
)

// CScalarKind classifies one compiler-known fundamental C ABI scalar.
type CScalarKind string

const (
	CScalarSigned   CScalarKind = "signed-integer"
	CScalarUnsigned CScalarKind = "unsigned-integer"
	CScalarFloat    CScalarKind = "floating"
	CScalarBool     CScalarKind = "bool"
)

// CScalar is the resolved representation of one C:: fundamental type under
// one C ABI model. ValueBits is the semantic precision; StorageBits is the
// physical object size, which exceeds ValueBits for x87 long double.
type CScalar struct {
	Name        string
	Kind        CScalarKind
	ValueBits   uint16
	StorageBits uint16
	Float       CFloatFormat
}

// CABIModel is the resolved C data representation of one target. It is part
// of the target definition carried by the CompilationPlan; consumers never
// derive C widths or signedness from architecture spellings.
//
// Rules:
//   - rules/platform/abi.md — § 19 "C scalar representation"; § 20 "C aggregates"
//   - rules/platform/ffi.md — §5 "Fundamental C ABI scalar family"; §49 "Target and ABI validation"
type CABIModel struct {
	Name              string
	CharSigned        bool
	ShortBits         uint16
	IntBits           uint16
	LongBits          uint16
	LongLongBits      uint16
	BoolBits          uint16
	LongDouble        CFloatFormat
	LongDoubleStorage uint16
	PointerWidthBits  uint16
	DataModelSpelling string
}

// CFundamentalTypeNames lists the compiler-known C:: fundamental scalar
// names in ffi.md §5 order.
func CFundamentalTypeNames() []string {
	return []string{
		"char", "schar", "uchar",
		"short", "ushort",
		"int", "uint",
		"long", "ulong",
		"long_long", "ulong_long",
		"float", "double", "long_double",
		"bool",
	}
}

// Defined reports whether the target supplies a C ABI model at all.
func (m CABIModel) Defined() bool { return m.Name != "" }

// Validate rejects an incoherent model before any consumer trusts it.
func (m CABIModel) Validate() error {
	if !m.Defined() {
		return nil
	}
	for _, width := range []uint16{m.ShortBits, m.IntBits, m.LongBits, m.LongLongBits, m.BoolBits} {
		if width == 0 || width%8 != 0 || width > 64 {
			return fmt.Errorf("C ABI model %s has invalid integer width %d", m.Name, width)
		}
	}
	if m.ShortBits > m.IntBits || m.IntBits > m.LongBits || m.LongBits > m.LongLongBits {
		return fmt.Errorf("C ABI model %s violates short <= int <= long <= long long", m.Name)
	}
	if m.LongDouble == cFloatFormatUnresolved || m.LongDoubleStorage < floatValueBits(m.LongDouble) {
		return fmt.Errorf("C ABI model %s has no valid long double representation", m.Name)
	}
	return nil
}

// Fundamental resolves one C:: fundamental name. Plain C char keeps its own
// identity while sharing the target-selected signedness of schar or uchar.
func (m CABIModel) Fundamental(name string) (CScalar, bool) {
	if !m.Defined() {
		return CScalar{}, false
	}
	integer := func(kind CScalarKind, bits uint16) (CScalar, bool) {
		return CScalar{Name: "C::" + name, Kind: kind, ValueBits: bits, StorageBits: bits}, true
	}
	switch name {
	case "char":
		if m.CharSigned {
			return integer(CScalarSigned, 8)
		}
		return integer(CScalarUnsigned, 8)
	case "schar":
		return integer(CScalarSigned, 8)
	case "uchar":
		return integer(CScalarUnsigned, 8)
	case "short":
		return integer(CScalarSigned, m.ShortBits)
	case "ushort":
		return integer(CScalarUnsigned, m.ShortBits)
	case "int":
		return integer(CScalarSigned, m.IntBits)
	case "uint":
		return integer(CScalarUnsigned, m.IntBits)
	case "long":
		return integer(CScalarSigned, m.LongBits)
	case "ulong":
		return integer(CScalarUnsigned, m.LongBits)
	case "long_long":
		return integer(CScalarSigned, m.LongLongBits)
	case "ulong_long":
		return integer(CScalarUnsigned, m.LongLongBits)
	case "float":
		return CScalar{Name: "C::float", Kind: CScalarFloat, ValueBits: 32, StorageBits: 32, Float: CFloatBinary32}, true
	case "double":
		return CScalar{Name: "C::double", Kind: CScalarFloat, ValueBits: 64, StorageBits: 64, Float: CFloatBinary64}, true
	case "long_double":
		return CScalar{Name: "C::long_double", Kind: CScalarFloat, ValueBits: floatValueBits(m.LongDouble), StorageBits: m.LongDoubleStorage, Float: m.LongDouble}, true
	case "bool":
		return CScalar{Name: "C::bool", Kind: CScalarBool, ValueBits: 1, StorageBits: m.BoolBits}, true
	}
	return CScalar{}, false
}

func floatValueBits(format CFloatFormat) uint16 {
	switch format {
	case CFloatBinary32:
		return 32
	case CFloatBinary64:
		return 64
	case CFloatX87Extended80:
		return 80
	case CFloatBinary128:
		return 128
	}
	return 0
}

// Canonical C data models used by the target table. LP64 Unix targets keep
// 64-bit long; LLP64 Windows keeps 32-bit long; ILP32 targets keep 32-bit
// long and pointers. Character signedness and long double format are selected
// per platform ABI.
func lp64CModel(name string, charSigned bool, longDouble CFloatFormat, longDoubleStorage uint16) CABIModel {
	return CABIModel{Name: name, CharSigned: charSigned, ShortBits: 16, IntBits: 32, LongBits: 64, LongLongBits: 64, BoolBits: 8, LongDouble: longDouble, LongDoubleStorage: longDoubleStorage, PointerWidthBits: 64, DataModelSpelling: "LP64"}
}

func llp64CModel(name string) CABIModel {
	return CABIModel{Name: name, CharSigned: true, ShortBits: 16, IntBits: 32, LongBits: 32, LongLongBits: 64, BoolBits: 8, LongDouble: CFloatBinary64, LongDoubleStorage: 64, PointerWidthBits: 64, DataModelSpelling: "LLP64"}
}

func ilp32CModel(name string, longDouble CFloatFormat, longDoubleStorage uint16) CABIModel {
	return CABIModel{Name: name, CharSigned: false, ShortBits: 16, IntBits: 32, LongBits: 32, LongLongBits: 64, BoolBits: 8, LongDouble: longDouble, LongDoubleStorage: longDoubleStorage, PointerWidthBits: 32, DataModelSpelling: "ILP32"}
}

// The named models below are the C ABIs of the currently registered targets.
var (
	CModelSysVX8664      = lp64CModel("sysv-x86_64", true, CFloatX87Extended80, 128)
	CModelAAPCS64        = lp64CModel("aapcs64", false, CFloatBinary128, 128)
	CModelDarwinX8664    = lp64CModel("darwin-x86_64", true, CFloatX87Extended80, 128)
	CModelDarwinARM64    = lp64CModel("darwin-arm64", true, CFloatBinary64, 64)
	CModelWindowsX64     = llp64CModel("windows-x64")
	CModelWindowsARM64   = llp64CModel("windows-arm64")
	CModelAAPCSLinuxHF   = ilp32CModel("aapcs-linux-gnueabihf", CFloatBinary64, 64)
	CModelAAPCSBareMetal = ilp32CModel("aapcs-eabi", CFloatBinary64, 64)
	CModelRISCVILP32     = ilp32CModel("riscv-ilp32", CFloatBinary128, 128)
	// RISC-V LP64D (psABI): unsigned char and IEEE binary128 long double.
	CModelRISCVLP64D = lp64CModel("riscv-lp64d", false, CFloatBinary128, 128)
	// FreeBSD armv7 uses the same hard-float EABI data model as Linux.
	CModelAAPCSFreeBSDHF = ilp32CModel("aapcs-freebsd-gnueabihf", CFloatBinary64, 64)
)
