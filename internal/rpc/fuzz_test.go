package rpc

import (
	"encoding/json"
	"testing"
)

func FuzzParseRPCAmount(f *testing.F) {
	f.Add("1.0")
	f.Add("0.0001")
	f.Add("-1")
	f.Add("abc")
	f.Fuzz(func(t *testing.T, v string) {
		raw, _ := json.Marshal(v)
		_, _ = parseRPCAmount(raw, false)
		_, _ = parseRPCAmount(raw, true)
	})
}

func FuzzParsePassphraseArg(f *testing.F) {
	f.Add("pass")
	f.Add("")
	f.Add("123")
	f.Fuzz(func(t *testing.T, v string) {
		raw, _ := json.Marshal(v)
		_, _ = parsePassphraseArg(raw)
	})
}

func FuzzParseSendManyOutputs(f *testing.F) {
	f.Add(`{"LExampleAddress1111111111111111111111":0.1}`)
	f.Add(`{}`)
	f.Add(`{"not-an-address":"x"}`)
	f.Fuzz(func(t *testing.T, raw string) {
		_, _ = parseSendManyOutputs(json.RawMessage(raw), false)
		_, _ = parseSendManyOutputs(json.RawMessage(raw), true)
	})
}
