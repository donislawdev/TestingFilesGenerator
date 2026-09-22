// The reference tools for the two configuration formats.
//
// Their own file rather than more of scripts.go, which was itself split out of
// oracle.go for the same reason: the guard on file size is what decides where
// a script lives, and it has decided twice now.
package oracle

// pythonYAMLScript reads the document back and checks that the records
// survived as records, rather than only that the file parsed.
//
// PyYAML rather than ruamel, because python3-yaml is the package a Debian
// runner and a tester's machine both already have. What that costs is written
// down rather than left to be discovered: measured on 2026-09-22, PyYAML
// ACCEPTS a duplicate key where ruamel, goccy and yaml.v3 all refuse one. So
// this reader is not a witness for that defect and the structural guard beside
// it carries that half.
//
// safe_load rather than load, and it matters here more than it reads: load
// builds arbitrary Python objects from a document, and a generator that ever
// emitted a tag would be running code inside its own oracle.
const pythonYAMLScript = `
import sys
try:
    import yaml
except ImportError:
    print("SKIP no pyyaml"); sys.exit(0)

with open(sys.argv[1], "rb") as fh:
    try:
        doc = yaml.safe_load(fh)
    except yaml.YAMLError as exc:
        print("FAIL", str(exc).replace("\n", " ")); sys.exit(1)

if not isinstance(doc, dict):
    print("FAIL the document is", type(doc).__name__, "and not a mapping"); sys.exit(1)
records = doc.get("records")
if not isinstance(records, list) or not records:
    print("FAIL the document carries no records"); sys.exit(1)
want = {"id": int, "name": str, "email": str, "amount": float,
        "active": bool, "tags": list, "address": dict, "note": str}
for i, rec in enumerate(records):
    if not isinstance(rec, dict):
        print("FAIL record", i + 1, "is", type(rec).__name__); sys.exit(1)
    for key, kind in want.items():
        if key not in rec:
            print("FAIL record", i + 1, "has no", key); sys.exit(1)
        if not isinstance(rec[key], kind):
            print("FAIL record", i + 1, key, "came back as",
                  type(rec[key]).__name__, "not", kind.__name__); sys.exit(1)
print("OK", len(records), "records")
`

// pythonTOMLScript does the same for TOML, through the standard library.
//
// tomllib rather than anything installed, because it has been in Python since
// 3.11 - so this is the one oracle in the project that cannot be missing. It
// reads bytes rather than text on purpose: that is the interface that decides
// the encoding the way the specification does, and measured on 2026-09-22 it
// is also the one that refuses a byte order mark, which TOML forbids.
const pythonTOMLScript = `
import sys, tomllib

with open(sys.argv[1], "rb") as fh:
    try:
        doc = tomllib.load(fh)
    except tomllib.TOMLDecodeError as exc:
        print("FAIL", str(exc).replace("\n", " ")); sys.exit(1)

records = doc.get("records")
if not isinstance(records, list) or not records:
    print("FAIL the document carries no records"); sys.exit(1)
want = {"id": int, "name": str, "email": str, "amount": float,
        "active": bool, "tags": list, "address": dict, "note": str}
for i, rec in enumerate(records):
    for key, kind in want.items():
        if key not in rec:
            print("FAIL record", i + 1, "has no", key); sys.exit(1)
        if not isinstance(rec[key], kind):
            print("FAIL record", i + 1, key, "came back as",
                  type(rec[key]).__name__, "not", kind.__name__); sys.exit(1)
print("OK", len(records), "records")
`
