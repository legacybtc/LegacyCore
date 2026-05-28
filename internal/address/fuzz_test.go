package address

import "testing"

func FuzzDecodeBase58AndHybrid(f *testing.F) {
	f.Add("LZMF6Jxj5K6YdABTKu7HqrV6x6qY8xL8bA")
	f.Add("lhyb1LZMF6Jxj5K6YdABTKu7HqrV6x6qY8xL8bA")
	f.Add("")
	f.Add("!!!!")
	f.Fuzz(func(t *testing.T, input string) {
		if version, payload, err := DecodeBase58Check(input); err == nil {
			round := EncodeBase58Check(version, payload)
			_, _, _ = DecodeBase58Check(round)
		}
		if payload, err := DecodeHybridAddress(input); err == nil {
			if len(payload) != HashSize {
				t.Fatalf("decoded hybrid payload size=%d want %d", len(payload), HashSize)
			}
		}
	})
}
