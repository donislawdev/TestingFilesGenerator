// Part of package cli. See cli.go.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// claimsOf reads what the manifest at path claims, for verify and cleanup,
// under the memory setting, and says on errOut why not when it cannot. When ok
// is false the command ends here with code.
//
// One function for both commands, so they read a manifest under the same
// limit and refuse it in the same words. stopped is the sentence for Ctrl+C
// arriving while the manifest is read, which is the one ending the two put
// differently - one checked nothing, the other removed nothing - and both say
// so, because a manifest of a million files takes seconds to read and a person
// who stopped it wants to know what happened to their directory.
func claimsOf(ctx context.Context, path string, memory core.Memory, stopped string, errOut io.Writer) (claims manifest.Claims, code int, ok bool) {
	limits := memory.On(core.MachineMemory)
	engine.HandToRuntime(limits)
	claims, err := manifest.ReadClaims(ctx, path, limits.Run)
	if err == nil {
		return claims, ExitOK, true
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintf(errOut, "tfg: %s\n", stopped)
		return manifest.Claims{}, ExitInterrupted, false
	}
	fmt.Fprintf(errOut, "tfg: %s\n", describeError(err))
	return manifest.Claims{}, classify(err), false
}
