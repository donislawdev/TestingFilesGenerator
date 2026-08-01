// Package version holds the tool version in exactly one place.
//
// Only the owner raises it. A major bump is also a statement about file
// hashes in other people's CI, so it is never routine tidying.
package version

// Version is written into every manifest. A manifest without it cannot
// explain a hash mismatch after an upgrade.
const Version = "0.0.0-dev"
