import os
import sys

import pytest
from conftest import tfg_binary

from testing_files_generator import TfgError
from testing_files_generator._run import KEPT_ERROR, _status, ended, run, run_to_file


def test_the_output_goes_to_a_new_file_only_its_owner_can_read(tmp_path):
    out = tmp_path / "plan.json"
    run_to_file(tfg_binary(), ["version"], str(out))
    assert out.read_text(encoding="utf-8").strip()
    if sys.platform != "win32":
        assert os.stat(out).st_mode & 0o777 == 0o600


def test_a_file_already_under_that_name_is_not_written_through(tmp_path):
    planted = tmp_path / "plan.json"
    planted.write_text("someone else's", encoding="utf-8")
    with pytest.raises(FileExistsError):
        run_to_file(tfg_binary(), ["version"], str(planted))
    assert planted.read_text(encoding="utf-8") == "someone else's"


def test_tfg_ending_with_a_code_is_an_error_with_the_code_and_what_tfg_said():
    with pytest.raises(TfgError) as caught:
        run(tfg_binary(), ["generate", "--no-such-flag"])
    assert caught.value.status == 2
    assert "flag provided but not defined: -no-such-flag" in caught.value.stderr
    assert str(caught.value).startswith("tfg generate --no-such-flag ended with code 2.\n")


def test_a_binary_that_cannot_start_is_named_with_what_to_do(tmp_path):
    with pytest.raises(RuntimeError) as caught:
        run(str(tmp_path / "no-such-tfg"), ["version"])
    said = str(caught.value)
    assert said.startswith(f"tfg could not be started from {tmp_path / 'no-such-tfg'}: ")
    assert "pip install --force-reinstall testing-files-generator" in said and "TFG_BINARY" in said


@pytest.mark.skipif(sys.platform == "win32", reason="the execute bit is a thing of Linux and macOS")
def test_a_binary_without_its_execute_bit_keeps_the_reason_the_system_gave(tmp_path):
    binary = tmp_path / "tfg"
    binary.write_bytes(b"#!/bin/sh\n")
    binary.chmod(0o644)
    with pytest.raises(RuntimeError) as caught:
        run(str(binary), ["version"])
    assert "Permission denied" in str(caught.value)
    assert "TFG_BINARY" in str(caught.value)


def test_a_signal_is_the_status_by_its_name_and_a_number_with_no_name_is_still_a_signal():
    assert _status(-15) == "SIGTERM"
    assert _status(-200) == "signal 200"
    assert _status(3) == 3
    assert ended("SIGTERM") == "was stopped by SIGTERM"
    assert ended(3) == "ended with code 3"


@pytest.mark.skipif(sys.platform == "win32", reason="Windows ends a process with a code, never a signal")
def test_tfg_stopped_by_a_signal_is_an_error_naming_the_signal():
    # Python stands in for tfg, so the run stops by a signal for certain.
    script = "import os, signal; os.kill(os.getpid(), signal.SIGTERM)"
    with pytest.raises(TfgError) as caught:
        run(sys.executable, ["-c", script])
    assert caught.value.status == "SIGTERM"
    assert str(caught.value).startswith(f"tfg -c {script} was stopped by SIGTERM.\n")


def test_only_the_end_of_a_long_error_stream_is_kept():
    # More than is kept, with the start and the end told apart, and a long
    # line that does not fit in a pipe's buffer.
    script = (f"import sys; sys.stderr.write('S' + 'x' * {KEPT_ERROR * 2} + 'the end'); sys.stderr.flush(); "
              "sys.exit(5)")
    with pytest.raises(TfgError) as caught:
        run(sys.executable, ["-c", script])
    assert caught.value.status == 5
    assert len(caught.value.stderr) == KEPT_ERROR
    assert caught.value.stderr.endswith("the end")
    assert "S" not in caught.value.stderr
