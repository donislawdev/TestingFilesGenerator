"""Which guards one part of the race job runs, and whether the weights still hold.

    race_parts.py split NAMES WEIGHTS PART PARTS > mine.txt
    race_parts.py unlisted WEIGHTS LOG

split prints the names of NAMES (one per line, as go test -list gives them)
that part PART of PARTS runs. The tests WEIGHTS names are laid out by their
weight, the heaviest first, each into the part that is lightest so far. Every
other test goes where the checksum of its name sends it and counts as the
"unlisted" weight, so adding or removing a light test moves no other test, and
the heavy ones are laid over the light ones rather than beside them.

unlisted reads the log of a part (go test -v) and fails when a test WEIGHTS
does not name took UNLISTED_CEILING seconds or more. Such a test is laid out as
if it were light, and the parts stop being even without anything saying so.

Both fail closed, with a sentence that names what to change: a weight for a
test the binary does not list (renamed or removed), a line that is not a
weight, a name given twice, no "unlisted" line, or a part outside the parts.
"""

import math
import re
import sys
import zlib

# The weights list every test that took 10 s or more on the slowest processor
# measured, so an unlisted test is under 10 s there, and no test measured took
# more than 2.7 times as long on one processor as on another. Six times the
# listing threshold is a test that is new or has grown, not one the machine
# made slow.
UNLISTED_CEILING = 60.0

TOP = re.compile(r"^--- (?:PASS|FAIL|SKIP): (\S+) \(([0-9.]+)s\)$")


def refuse(message):
    print(message, file=sys.stderr)
    sys.exit(1)


def read_lines(path, what):
    try:
        with open(path, encoding="utf-8") as f:
            return f.read().splitlines()
    except (OSError, UnicodeDecodeError) as err:
        refuse("cannot read the %s %s: %s" % (what, path, err))


def read_weights(path):
    unlisted = None
    weights = {}
    for number, line in enumerate(read_lines(path, "weights"), 1):
        text = line.strip()
        if not text or text.startswith("#"):
            continue
        fields = text.split()
        where = "%s line %d (%s)" % (path, number, text)
        if len(fields) != 2:
            refuse("%s is not a weight. Each line is seconds and a test name, or \"unlisted\" and seconds." % where)
        if fields[0] == "unlisted":
            if unlisted is not None:
                refuse("%s gives the unlisted weight a second time." % where)
            unlisted = seconds(fields[1], where)
            continue
        took, name = seconds(fields[0], where), fields[1]
        if name in weights:
            refuse("%s names %s a second time. Keep one line for it." % (where, name))
        weights[name] = took
    if unlisted is None:
        refuse("%s has no \"unlisted\" line, so the tests it does not name weigh nothing. "
               "Add one with the mean time of those tests." % path)
    return unlisted, weights


def seconds(text, where):
    try:
        value = float(text)
    except ValueError:
        value = math.nan
    if not math.isfinite(value) or value < 0:
        refuse("%s gives %s, and a weight is a number of seconds, nought or more." % (where, text))
    return value


def lay_out(names, unlisted, weights, parts):
    """Return the part of every name and the weight each part carries."""
    gone = [name for name in weights if name not in names]
    if gone:
        refuse("the weights name %s, which the test binary does not list - a test renamed or removed. "
               "Take the line out of the weights, or give it the test's new name." % ", ".join(sorted(gone)))
    part_of = {}
    load = [0.0] * parts
    for name in names:
        if name not in weights:
            part = zlib.crc32(name.encode("utf-8")) % parts
            part_of[name] = part
            load[part] += unlisted
    for name in sorted(weights, key=lambda n: (-weights[n], n)):
        part = min(range(parts), key=lambda p: (load[p], p))
        part_of[name] = part
        load[part] += weights[name]
    return part_of, load


def split(names_path, weights_path, part_text, parts_text):
    try:
        part, parts = int(part_text), int(parts_text)
    except ValueError:
        refuse("the part and the number of parts are whole numbers, and these are %s and %s." % (part_text, parts_text))
    if parts < 1 or not 0 <= part < parts:
        refuse("part %d of %d does not exist. Parts count from nought and stay below their number." % (part, parts))
    names = list(dict.fromkeys(n.strip() for n in read_lines(names_path, "test list") if n.strip()))
    unlisted, weights = read_weights(weights_path)
    part_of, load = lay_out(set(names), unlisted, weights, parts)
    mine = [name for name in names if part_of[name] == part]
    for name in mine:
        print(name)
    print("part %d of %d runs %d of %d tests, about %.0f s by the weights (the parts: %s s)" % (
          part, parts, len(mine), len(names), load[part], ", ".join("%.0f" % l for l in load)), file=sys.stderr)


def check_unlisted(weights_path, log_path):
    _, weights = read_weights(weights_path)
    long = []
    for line in read_lines(log_path, "log"):
        found = TOP.match(line)
        if found and found.group(1) not in weights and float(found.group(2)) >= UNLISTED_CEILING:
            long.append((found.group(1), float(found.group(2))))
    if long:
        lines = "\n".join("  %.1f %s" % (took, name) for name, took in long)
        refuse("these tests took %.0f s or more and the weights do not name them, so the parts are laid out "
               "as if they were light:\n%s\nAdd each line above to %s." % (UNLISTED_CEILING, lines, weights_path))


def main():
    args = sys.argv[1:]
    if len(args) == 5 and args[0] == "split":
        split(*args[1:])
    elif len(args) == 3 and args[0] == "unlisted":
        check_unlisted(*args[1:])
    else:
        refuse("usage: race_parts.py split NAMES WEIGHTS PART PARTS | race_parts.py unlisted WEIGHTS LOG")


if __name__ == "__main__":
    main()
