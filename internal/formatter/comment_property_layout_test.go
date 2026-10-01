package formatter

import "testing"

// A block comment whose opener is indented further than its body lines keeps
// its aligned-star text: existing stars are not repeated, plain lines are not
// mistaken for preformatted text, and real preformatted lines survive.
//
// Rules:
//   - rules/tooling/formatter.md — §12(8) aligned-star form, §12(12) surrounding indentation, §12(14) preformatted preservation
func TestFormatShiftedBlockCommentOpener(t *testing.T) {
	input := "module main\n\nimpl S {\n" +
		"        /*\n    * a\n    * b\n    */\n" +
		"            /**\n     * Doc\n     *\n     *       preformatted\n     */\n" +
		"        /*\n    plain\n        code\n    */\n" +
		"    fn F() int {\n        return 1\n    }\n}\n"
	want := "module main\n\nimpl S {\n" +
		"    /*\n     * a\n     * b\n     */\n" +
		"    /**\n     * Doc\n     *\n     *       preformatted\n     */\n" +
		"    /*\n     * plain\n     *     code\n     */\n" +
		"    fn F() int {\n        return 1\n    }\n}\n"
	assertFormat(t, input, want)
}

// Compressed property accessor layouts are expanded to one accessor per line
// with the property's closing brace on its own line, so the following members
// and declarations keep their indentation; a fallible setter keeps try set
// together.
//
// Rules:
//   - rules/tooling/formatter.md — §16(7) property layout, §16(9) fallible setter, §8(4) executable blocks
func TestFormatCompressedPropertyAccessors(t *testing.T) {
	input := "module main\n\nimpl S {\n" +
		"    property X: int { get {\n        return a\n    } }\n\n" +
		"    property Y: int {\n        get {\n            return a\n        } }\n\n" +
		"    property Z: int { get { return a } }\n\n" +
		"    property W: int { get {\n        return a\n    } try set value {\n        a = value\n    } }\n\n" +
		"    fn F() int {\n        return 1\n    }\n}\n\nfn G() int {\n    return 2\n}\n"
	getter := "        get {\n            return a\n        }\n"
	want := "module main\n\nimpl S {\n" +
		"    property X: int {\n" + getter + "    }\n\n" +
		"    property Y: int {\n" + getter + "    }\n\n" +
		"    property Z: int {\n" + getter + "    }\n\n" +
		"    property W: int {\n" + getter + "        try set value {\n            a = value\n        }\n    }\n\n" +
		"    fn F() int {\n        return 1\n    }\n}\n\nfn G() int {\n    return 2\n}\n"
	assertFormat(t, input, want)
}

// Bodyless interface property requirements are not accessor blocks and keep
// their compact form.
//
// Rules:
//   - rules/tooling/formatter.md — §16(7) property layout
func TestFormatPropertyRequirementStaysCompact(t *testing.T) {
	input := "module main\n\ninterface Named {\n    property Name: string { get }\n}\n"
	assertFormat(t, input, input)
}
