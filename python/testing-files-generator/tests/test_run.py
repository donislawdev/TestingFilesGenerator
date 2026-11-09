import os
import sys

import pytest
from conftest import tfg_binary

from testing_files_generator import TfgError
from testing_files_generator._run import run, run_to_file


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


def test_a_binary_that_cannot_start_is_named(tmp_path):
    with pytest.raises(RuntimeError, match="tfg could not be started from"):
        run(str(tmp_path / "no-such-tfg"), ["version"])
