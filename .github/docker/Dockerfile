# The command line as a container image: ghcr.io/donislawdev/tfg.
#
# Not built from source. The build context is the Linux archive of a PUBLISHED
# release, unpacked by .github/workflows/docker.yml only after its checksum and
# its attestation checked out. So the program in the image is, byte for byte,
# the one a person downloads, and it writes the same files: measured
# 2026-10-20, eight formats from the amd64 and the arm64 image against the
# Windows build of the same commit, every checksum equal.
#
# Nothing else is in it. The command line is static, never opens a connection
# (D16) and reads no home directory, temporary directory, user database or time
# zone, so there is no operating system to put under it.
FROM scratch

COPY tfg /tfg

# The notices travel with the program because their licences require it, the
# same two files every release archive carries.
COPY LICENSE THIRD-PARTY-NOTICES.md /licenses/

# Files go where the caller mounts a directory. /work belongs to root and the
# program runs as somebody else, so a run with nothing mounted is refused with
# exit code 5 instead of writing files into a container that --rm then throws
# away without a word.
WORKDIR /work
USER 65532:65532

ENTRYPOINT ["/tfg"]
