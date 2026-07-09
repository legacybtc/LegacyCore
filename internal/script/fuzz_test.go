package script

import "testing"

func FuzzValidateScriptStructure(f *testing.F) {
	f.Add([]byte{OP_DUP, OP_HASH160, 20})
	f.Add([]byte{OP_0})
	f.Add([]byte{OP_PUSHDATA1, 1, 0x01})
	f.Fuzz(func(t *testing.T, program []byte) {
		_ = ValidateScriptStructure(program)
		_, _ = EvalPushScript(program, nil)
		_ = CountSigOps(program)
	})
}
