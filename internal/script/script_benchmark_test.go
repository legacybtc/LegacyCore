package script

import "testing"

func BenchmarkValidateScriptStructureP2PKH(b *testing.B) {
	program := []byte{
		OP_DUP,
		OP_HASH160,
		0x14,
		0x11, 0x11, 0x11, 0x11, 0x11,
		0x11, 0x11, 0x11, 0x11, 0x11,
		0x11, 0x11, 0x11, 0x11, 0x11,
		0x11, 0x11, 0x11, 0x11, 0x11,
		OP_EQUALVERIFY,
		OP_CHECKSIG,
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := ValidateScriptStructure(program); err != nil {
			b.Fatalf("ValidateScriptStructure: %v", err)
		}
	}
}

func BenchmarkValidateScriptStructureMalformed(b *testing.B) {
	program := []byte{0x02, 0x01}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = ValidateScriptStructure(program)
	}
}
