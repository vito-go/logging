package tid

import (
	"testing"
)

func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		Get()
	}
}

// goos: linux
// goarch: amd64
// pkg: tid
// cpu: AMD Ryzen 9 3900X 12-Core Processor
// BenchmarkGet-24    	21152655	        50.81 ns/op	       0 B/op	       0 allocs/op
// BenchmarkGet-24    	22063012	        52.12 ns/op	       0 B/op	       0 allocs/op
// BenchmarkGet-24    	21347892	        49.78 ns/op	       0 B/op	       0 allocs/op
// BenchmarkGet-24    	21110130	        52.82 ns/op	       0 B/op	       0 allocs/op
// BenchmarkGet-24    	20710473	        50.33 ns/op	       0 B/op	       0 allocs/op
// PASS
// ok  	tid	5.743s
