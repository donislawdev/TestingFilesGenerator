package guard

import "testing"

// A data race put here on purpose, for one pull request only: the part of the
// race detector job that holds this test has to go red, and the other parts
// green. Reverted by the next commit, before the pull request is merged.
func TestADataRaceOnPurposeForTheRaceJob(t *testing.T) {
	n := 0
	done := make(chan struct{})
	go func() {
		n++
		close(done)
	}()
	n++
	<-done
	if n != 2 {
		t.Logf("the two increments landed as %d", n)
	}
}
