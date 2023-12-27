package cmd

import (
	"os"
	"testing"
)

func BenchmarkDiskusage(b *testing.B) {
	os.Stderr, _ = os.Open(os.DevNull)
	for i := 0; i < b.N; i++ {
		diskUsage(".")
	}
}

func BenchmarkDiskusagePipelined(b *testing.B) {
	os.Stderr, _ = os.Open(os.DevNull)
	for i := 0; i < b.N; i++ {
		diskUsagePipelined(".")
	}
}

func BenchmarkDiskusageWalkDir(b *testing.B) {
	os.Stderr, _ = os.Open(os.DevNull)
	for i := 0; i < b.N; i++ {
		diskUsageWalkDir(".")
	}
}
