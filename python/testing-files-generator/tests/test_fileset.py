import json
import os
import subprocess
import sys
import time
import warnings

import pytest
from conftest import SMALL, SRC, python, tfg_binary, writer

from testing_files_generator import TfgError, UnfinishedSetWarning, file_set
from testing_files_generator import _fileset


def planned(cache, spec=None):
    return file_set(**(spec or SMALL), binary=tfg_binary(), cache_dir=cache)


def entries(cache):
    return sorted(os.listdir(cache))


def test_a_plan_writes_nothing_and_says_every_file_with_what_is_expected_of_it(cache):
    s = planned(cache)
    assert entries(cache) == [], "a plan left something in the cache"
    assert len(s.files) == s.plan["summary"]["file_count"]
    assert len(s.files) >= 20, f"the set this test asks for holds {len(s.files)} files"
    assert len({f.title for f in s.files}) == len(s.files), "two files share a title"
    assert len(s.only(outcome="unspecified")) == s.plan["summary"]["by_expected"]["unspecified"]
    assert len(s.only(group="bulk-upload")) == 2
    assert len(s.except_(group="bulk-upload")) == len(s) - 2
    assert len(s.only(group=["bulk-upload", "size-limit"], outcome="accept")) > 0


def test_a_target_or_a_list_of_them_picks_the_files_made_for_it(cache):
    s = planned(cache)
    targets = sorted({f.target for f in s.files})
    assert len(targets) >= 3, f"the set this test asks for has the targets {targets}"
    one, two = targets[0], targets[1]
    made_for_one = [f for f in s.files if f.target == one]
    assert list(s.only(target=one)) == made_for_one
    assert list(s.except_(target=one)) == [f for f in s.files if f.target != one]
    assert {f.target for f in s.only(target=[one, two])} == {one, two}
    assert list(s.only(target=one, outcome=made_for_one[0].expected["outcome"])) == \
        [f for f in made_for_one if f.expected["outcome"] == made_for_one[0].expected["outcome"]]


def test_a_set_is_written_once_whole_and_its_files_are_where_their_paths_say(cache):
    s = planned(cache)
    directory = s.write()
    assert s.write() == directory
    assert entries(cache) == [os.path.basename(directory)]
    for f in s.files:
        assert os.path.getsize(f.path()) == f.bytes, f.relative_path
    assert s.only(group="bulk-upload").paths() == [os.path.join(directory, f.relative_path)
                                                   for f in s.only(group="bulk-upload")]


# A racer plans a set, waits until every racer has planned, and only then
# writes, so that all of them find the set unwritten and write it at once. A
# racer that fails before it reaches the barrier would hold the others there
# for ever, so the wait has an end.
RACER = """
import json, os, sys, time
from testing_files_generator import _fileset, file_set
said = []
_fileset._listeners.append(lambda event, path: said.append(event))
spec, barrier, racers = json.loads(sys.argv[1]), sys.argv[2], int(sys.argv[3])
s = file_set(**spec)
open(os.path.join(barrier, str(os.getpid())), "w").close()
until = time.monotonic() + 30
while len(os.listdir(barrier)) < racers:
    if time.monotonic() > until:
        sys.exit("not every racer planned within 30 s")
    time.sleep(0.005)
print(json.dumps({"dir": s.write(), "said": said}))
"""


def test_workers_writing_one_set_at_once_end_with_one_copy_and_nothing_left_over(cache, tmp_path):
    barrier = tmp_path / "barrier"
    barrier.mkdir()
    racers = 4
    env = {**os.environ, "TFG_BINARY": tfg_binary(), "PYTHONPATH": SRC}
    spec = json.dumps({**SMALL, "cache_dir": str(cache)})
    children = [subprocess.Popen([sys.executable, "-c", RACER, spec, str(barrier), str(racers)], env=env,
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, encoding="utf-8")
                for _ in range(racers)]
    runs = [(c.wait(timeout=120), *c.communicate()) for c in children]
    # Every failed racer is shown, because the first to fail may only be one
    # that waited for another.
    assert [err for code, _, err in runs if code != 0] == [], "a racer failed"
    results = [json.loads(out) for _, out, _ in runs]
    said = [e for r in results for e in r["said"]]
    assert said.count("wrote") == racers, f"not every worker wrote, so there was no race: {said}"
    assert said.count("moved") == 1, said
    assert said.count("took") == racers - 1, said
    dirs = {r["dir"] for r in results}
    assert len(dirs) == 1, f"the workers were given {dirs}"
    assert entries(cache) == [os.path.basename(next(iter(dirs)))]
    assert planned(cache).write() == next(iter(dirs))


def test_a_later_run_takes_the_set_an_earlier_one_wrote(cache):
    first = writer(cache)
    assert first.returncode == 0, first.stderr
    directory = first.stdout.strip()
    before = os.stat(os.path.join(directory, "manifest.json")).st_mtime_ns
    second = writer(cache)
    assert second.stdout.strip() == directory
    assert os.stat(os.path.join(directory, "manifest.json")).st_mtime_ns == before, "the set was written again"


def test_another_seed_or_clean_is_another_set_though_the_recipe_hash_is_the_same(cache):
    plain = planned(cache)
    seeded = planned(cache, {**SMALL, "options": {**SMALL["options"], "seed": 7}})
    clean = planned(cache, {**SMALL, "options": {**SMALL["options"], "clean": True}})
    assert seeded.plan["run"]["recipe_hash"] == plain.plan["run"]["recipe_hash"], \
        "the hash now covers the seed - this test is no longer about anything"
    assert len({plain.write(), seeded.write(), clean.write()}) == 3


def test_an_option_written_with_underscores_is_the_flag_with_hyphens(cache):
    hyphens = planned(cache)
    underscores = planned(cache, {**SMALL, "options": {"limit": "1mb", "bulk": 2, "far_over": "off"}})
    assert underscores.write() == hyphens.write()


def test_a_set_changed_by_hand_is_refused_and_left_where_it_is(cache):
    directory = writer(cache).stdout.strip()
    os.remove(os.path.join(directory, "1mb_at_limit.jpg"))
    second = writer(cache)
    assert second.returncode == 3
    assert "does not hold it - 1mb_at_limit.jpg is missing. Nothing here removes that directory" in second.stderr
    assert os.path.exists(os.path.join(directory, "manifest.json")), "the changed set was removed"


def test_a_kept_manifest_whose_file_is_not_an_object_is_refused_like_any_changed_set(cache):
    directory = writer(cache).stdout.strip()
    manifest = os.path.join(directory, "manifest.json")
    with open(manifest, encoding="utf-8") as f:
        written = json.load(f)
    first = written["files"][0]["path"]
    written["files"][0] = None
    with open(manifest, "w", encoding="utf-8") as f:
        json.dump(written, f)
    again = writer(cache)
    assert again.returncode == 3
    assert (f"does not hold it - its file 1 is not described by an object where the plan has {first}. "
            "Nothing here removes that directory") in again.stderr, again.stderr


def test_a_file_of_the_set_cut_short_by_hand_is_refused(cache):
    directory = writer(cache).stdout.strip()
    os.truncate(os.path.join(directory, "1mb_at_limit.jpg"), 10)
    again = writer(cache)
    assert again.returncode == 3
    assert "1mb_at_limit.jpg is 10 B and its manifest says 1048576 B" in again.stderr


def test_a_file_written_over_by_hand_its_size_the_same_is_refused(cache):
    directory = writer(cache).stdout.strip()
    changed = os.path.join(directory, "1mb_at_limit.jpg")
    with open(changed, "r+b") as f:
        f.write(b"changed")
    again = writer(cache)
    assert again.returncode == 3, again.stderr
    assert "its files are not the ones it was written with. Nothing here removes that directory" in again.stderr
    assert "wrong-hash" in again.stderr and "1mb_at_limit.jpg" in again.stderr
    assert os.path.exists(changed), "the changed set was removed"


def test_a_binary_or_a_recipe_that_is_not_there_is_named_with_what_to_do(cache):
    missing = os.path.join(str(cache), "no-such-tfg")
    with pytest.raises(FileNotFoundError) as caught:
        file_set(**SMALL, binary=missing, cache_dir=cache)
    assert f"The binary option names {missing}, and there is no file there." in str(caught.value)
    assert "leave it out" in str(caught.value)
    with pytest.raises(FileNotFoundError) as caught:
        file_set(recipe="no-such-recipe.yaml", binary=tfg_binary(), cache_dir=cache)
    assert os.path.abspath("no-such-recipe.yaml") in str(caught.value)
    assert "Path(__file__).parent" in str(caught.value)


def test_a_recipe_not_in_utf8_is_refused_by_tfg_in_its_own_words(cache, tmp_path):
    # Read here only for its hash, so what is wrong with it is said by tfg,
    # which reads it, and not by a decoder on the way.
    recipe = tmp_path / "latin.yaml"
    recipe.write_bytes(b"version: 1\n# caf\xe9\n")
    with pytest.raises(TfgError) as caught:
        file_set(recipe=recipe, binary=tfg_binary(), cache_dir=cache)
    assert "is not valid UTF-8" in str(caught.value) and "Save the file as UTF-8" in str(caught.value)


def test_an_edited_recipe_is_planned_again_in_the_same_process(cache, tmp_path):
    recipe = tmp_path / "set.yaml"
    recipe.write_text("version: 1\ntargets:\n  - id: one\n    format: txt\n    size: 1kb\n", encoding="utf-8")
    first = file_set(recipe=recipe, binary=tfg_binary(), cache_dir=cache)
    recipe.write_text("version: 1\ntargets:\n  - id: one\n    format: txt\n    size: 2kb\n", encoding="utf-8")
    second = file_set(recipe=recipe, binary=tfg_binary(), cache_dir=cache)
    assert [f.bytes for f in first] == [1024]
    assert [f.bytes for f in second] == [2048]


def test_a_cache_directory_that_cannot_be_made_says_how_to_give_another(tmp_path):
    taken = tmp_path / "a-file"
    taken.write_text("", encoding="utf-8")
    with pytest.raises(RuntimeError) as caught:
        file_set(**SMALL, binary=tfg_binary(), cache_dir=taken)
    said = str(caught.value)
    assert said.startswith(f"Sets are kept in {taken}, and it could not be made a directory - ")
    assert "TFG_CACHE_DIR" in said and "cache_dir" in said


def test_a_set_a_stopped_run_left_behind_is_named_once_outside_the_workers_and_never_used(cache):
    gone = subprocess.Popen([sys.executable, "-c", ""])
    gone.wait()
    cache.mkdir()
    left = cache / f".tmp-{gone.pid}-0bad0bad"
    left.mkdir()
    plan = (f"import warnings; warnings.simplefilter('always'); from testing_files_generator import file_set; "
            f"file_set(**{SMALL!r}, cache_dir={str(cache)!r})")
    runner = python(plan, env={"PYTEST_XDIST_WORKER": None})
    assert runner.returncode == 0, runner.stderr
    worker = python(plan, env={"PYTEST_XDIST_WORKER": "gw0"})
    assert worker.returncode == 0, worker.stderr
    assert worker.stderr == "", "a worker spoke about the cache"
    assert f"1 unfinished set was left in {cache}" in runner.stderr
    assert f".tmp-{gone.pid}-0bad0bad. Nothing uses them" in runner.stderr
    assert "UnfinishedSetWarning" in runner.stderr
    assert left.exists(), "the unfinished set was removed"


def test_a_process_that_ended_is_not_alive_and_this_one_is():
    gone = subprocess.Popen([sys.executable, "-c", ""])
    gone.wait()
    time.sleep(0.1)
    assert _fileset._alive(os.getpid())
    assert not _fileset._alive(gone.pid)


def test_what_tfg_refuses_reaches_the_test_with_its_own_words_and_code(cache):
    with pytest.raises(TfgError) as caught:
        planned(cache, {"preset": "upload-validation", "options": {"limit": "1kb"}})
    assert caught.value.status == 4
    assert "smallest PDF" in str(caught.value)


def test_a_refused_plan_is_told_in_the_words_of_the_call_without_the_flags_this_package_adds(cache):
    with pytest.raises(TfgError) as caught:
        planned(cache, {"preset": "no-such-preset"})
    message = str(caught.value)
    assert caught.value.status == 2
    assert message.startswith("file_set(preset='no-such-preset') could not be planned - tfg ended with code 2.\n")
    assert 'unknown preset "no-such-preset"' in message
    for added in ("dry-run", "--json", ".tmp-"):
        assert added not in message


def test_an_option_tfg_does_not_know_is_named_with_the_flags_tfg_has(cache):
    with pytest.raises(TfgError) as caught:
        planned(cache, {**SMALL, "options": {**SMALL["options"], "limt": "2mb"}})
    err = caught.value
    assert err.status == 2
    assert "gives the option limt, and tfg generate has no flag of that name." in str(err)
    assert "flag provided" not in str(err), "the help of tfg is in the message"
    assert "flag provided but not defined: -limt" in err.stderr
    taken = str(err).split("Its flags are ", 1)[1].split(". Not every one goes with every set", 1)[0].split(", ")
    for name in ("limit", "far-over", "bulk", "seed"):
        assert name in taken
    for name in ("out", "json", "dry-run", "preset"):
        assert name not in taken, f"{name} is set by this package and is offered as an option"
    assert "tfg preset show upload-validation" in str(err)


def test_a_call_that_names_no_files_to_write_is_told_so_before_tfg_runs(cache):
    for spec in ({}, {"options": {}}, {"options": {"clean": False}}, {"options": {"set": []}}):
        with pytest.raises(ValueError, match="file_set was not told which files to write"):
            file_set(**spec, binary=tfg_binary(), cache_dir=cache)
    assert not cache.exists(), "something ran"


def test_the_flags_this_package_sets_itself_cannot_be_set_again(cache):
    for flag in ("out", "json", "dry-run", "dry_run", "preset"):
        with pytest.raises(ValueError, match=f"The option {flag} cannot be set here"):
            planned(cache, {"preset": "upload-validation", "options": {flag: "x"}})
    with pytest.raises(ValueError, match="not a flag tfg has"):
        planned(cache, {"preset": "upload-validation", "options": {"--limit": "1mb"}})
    with pytest.raises(TypeError):
        planned(cache, {"preset": "upload-validation", "options": {"limit": {"mb": 1}}})
    with pytest.raises(TypeError, match="options takes a dict"):
        planned(cache, {"preset": "upload-validation", "options": ["limit=1mb"]})


def test_a_manifest_of_another_major_or_an_outcome_this_package_does_not_know_is_refused():
    plan = {"manifest_version": "1.0", "files": [{"path": "a", "expected": {"outcome": "accept"}}]}
    _fileset.readable(plan)
    with pytest.raises(RuntimeError, match="manifest of version 2.0, and this package reads version 1"):
        _fileset.readable({**plan, "manifest_version": "2.0"})
    with pytest.raises(RuntimeError, match='"quarantine", which this package does not know'):
        _fileset.readable({**plan, "files": [{"path": "a", "expected": {"outcome": "quarantine"}}]})


def test_sets_are_kept_in_a_directory_of_the_users_own_never_the_shared_temporary_one():
    user = _fileset._user_cache_dir
    # Homes made up for the test, in no real profile.
    assert user(env={"LOCALAPPDATA": r"D:\Local"}, system="win32", home=r"D:\profile") == \
        r"D:\Local\testing-files-generator"
    assert user(env={}, system="win32", home=r"D:\profile") == r"D:\profile\AppData\Local\testing-files-generator"
    assert user(env={}, system="darwin", home="/srv/profile") == "/srv/profile/Library/Caches/testing-files-generator"
    assert user(env={"XDG_CACHE_HOME": "/xdg"}, system="linux", home="/srv/profile") == "/xdg/testing-files-generator"
    # A relative XDG_CACHE_HOME is to be ignored, by the XDG specification.
    assert user(env={"XDG_CACHE_HOME": "rel"}, system="linux", home="/srv/profile") == \
        "/srv/profile/.cache/testing-files-generator"
    with pytest.raises(RuntimeError, match="found no home directory") as caught:
        user(env={}, system="linux", home="~")
    assert "TFG_CACHE_DIR" in str(caught.value) and "cache_dir" in str(caught.value)


def test_tfg_cache_dir_comes_first_and_then_the_cache_of_the_pytest_run(monkeypatch, tmp_path, pytestconfig):
    monkeypatch.setenv("TFG_CACHE_DIR", str(tmp_path))
    assert _fileset.default_cache_dir() == str(tmp_path)
    monkeypatch.delenv("TFG_CACHE_DIR")
    assert _fileset._pytest_config is pytestconfig, "the plugin did not tell the package about this run"
    if getattr(pytestconfig, "cache", None) is None:
        # Run with -p no:cacheprovider: there is no cache of the run to keep sets in.
        assert _fileset.default_cache_dir() == _fileset._user_cache_dir()
    else:
        assert _fileset.default_cache_dir() == str(pytestconfig.cache.mkdir("testing-files-generator"))


def test_no_warning_is_given_for_a_cache_with_nothing_left_behind(cache, monkeypatch):
    monkeypatch.setattr(_fileset, "_noted", False)
    with warnings.catch_warnings():
        warnings.simplefilter("error", UnfinishedSetWarning)
        planned(cache)
