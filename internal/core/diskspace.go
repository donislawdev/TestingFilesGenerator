package core

import "math"

// AvailableFrom turns what the system said about free space into a number of
// bytes this program can work with.
//
// The two platform files below both had the same shape at the end of them: a
// number the system reports as unsigned, multiplied or converted into an int64
// and handed on. On a filesystem of eight exbibytes or more that conversion
// wraps and free space comes back NEGATIVE, which the free space check reads as
// a disk smaller than any run - so the tool would refuse to write to the
// largest disk it will ever meet.
//
// Nobody has such a disk. It is here as a pure function rather than as a
// condition inside the syscall because a condition in there is one nothing
// could ever turn red, and this project has taken seven of those back out. As a
// function it can be asked directly, which is what a guard does.
//
// Saturating rather than erroring, and that is the truthful answer rather than
// the convenient one: a disk too large to count in int64 is a disk with more
// room than any run this tool can plan, so the largest number is what the
// caller should hear. An error would be read as "this disk cannot be measured",
// which is a different thing and is already what a failing syscall means.
func AvailableFrom(blocks, blockSize uint64) int64 {
	if blockSize == 0 {
		// A block size of nought would make the multiplication below divide by
		// nought in its own overflow check. No filesystem reports it, and the
		// answer if one did is that we know nothing about the space rather
		// than that there is none - but nought is what a failing syscall
		// already returns beside its error, so it is what this returns too.
		return 0
	}
	if blocks > uint64(math.MaxInt64)/blockSize {
		return math.MaxInt64
	}
	return int64(blocks * blockSize)
}
