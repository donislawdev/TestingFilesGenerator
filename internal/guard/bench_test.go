package guard

import (
	"context"
	"io"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// Nothing here had ever been timed. There are two guards on memory - one on
// what a generator allocates, one on what a plan costs per file - and none at
// all on how long anything takes.
//
// These write to io.Discard on purpose. A benchmark that writes to a disk
// measures the disk, and on this machine it would also feed the shadow copy
// service, which does not give the space back. What is wanted first is the
// cost of producing the bytes.
//
// Benchmarks do not run during `go test`, so they cost CI nothing:
//
//	go test ./internal/guard/ -run XXX -bench . -benchtime 3x

const benchSize = 16 << 20 // 16 MiB, big enough to drown the setup

func BenchmarkGenerate(b *testing.B) {
	for _, d := range format.All() {
		b.Run(d.ID, func(b *testing.B) {
			plan, err := d.Generator.Plan(format.Request{Bytes: benchSize, Seed: 7741, Label: true})
			if err != nil {
				b.Fatalf("planning: %v", err)
			}
			b.SetBytes(benchSize)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := d.Generator.Write(context.Background(), io.Discard, plan); err != nil {
					b.Fatalf("writing: %v", err)
				}
			}
		})
	}
}

// BenchmarkPlanTenThousand times the phase that happens before any byte is
// written. It is the one that decides how long `--dry-run` takes, and it is
// also the one holding the whole plan in memory.
func BenchmarkPlanTenThousand(b *testing.B) {
	targets := []engine.Target{{
		ID: "many", Format: "txt", Sizes: engine.Uniform(10000, 1024), Label: true,
	}}
	opt := engine.Options{OutDir: b.TempDir(), Seed: 7741, Command: "bench"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := engine.Plan(targets, opt); err != nil {
			b.Fatalf("planning: %v", err)
		}
	}
}
