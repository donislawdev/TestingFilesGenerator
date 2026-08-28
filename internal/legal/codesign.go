// The certificate this project signs Windows binaries with, pinned by the
// digest of its own bytes.
//
// The key is on a cryptographic card in a USB reader and cannot be exported.
// That is the whole value of it and the reason signing is a step a person runs
// rather than a job on a runner: no GitHub hosted runner can reach the card, a
// self hosted one could, and this is a public repository where a self hosted
// runner is a machine strangers can aim a pull request at.
//
// What is written here is the digest and nothing else. The certificate's own
// subject carries the owner's town and region - it is embedded in every file it
// signs and becomes public the first time a signed release goes out, which is a
// thing to say out loud once rather than to spread through a repository.

package legal

// CodeSigningSHA256 is the SHA-256 of the signing certificate's DER bytes.
//
// Read from the card on 2026-08-28: "Open Source Developer Dominik Babiarz",
// issued by Certum Code Signing 2021 CA, valid until 2027-08-19.
//
// Why this digest rather than the SHA-1 thumbprint Windows selects by: the
// thumbprint is what signtool takes as a selector and SHA-1 is not a digest
// worth pinning anything to. The script resolves one to the other at signing
// time, so the two cannot drift apart in a configuration file.
//
// A renewal is a DIFFERENT certificate and this line has to move with it.
// The signing script refuses when the certificate that actually signed a file
// does not hash to this, which is exactly the accident worth catching: a second
// code signing certificate on the same machine signs just as willingly, and the
// release page looks identical either way.
const CodeSigningSHA256 = "47b79ad3cfa53ef846cad03a59148f8c981d0b1196891e48b8c8d7982b10c148"

// CodeSigningExpires is when that certificate stops being able to sign.
//
// Written down so a check can warn before it happens rather than after. The
// signature outlives it - every signature this project makes carries an RFC
// 3161 timestamp, and without one a signature dies with its certificate - but
// nothing new can be signed past this date.
const CodeSigningExpires = "2027-08-19"
