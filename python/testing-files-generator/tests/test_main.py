import subprocess

from conftest import python, tfg_binary

TFG = "import sys; from testing_files_generator.__main__ import main; sys.argv[0] = 'tfg'; sys.exit(main())"


def test_tfg_from_the_package_is_tfg_with_the_same_output_and_exit_code():
    args = ["generate", "--format", "nope", "--dry-run"]
    via_package = python(TFG, *args)
    direct = subprocess.run([tfg_binary(), *args], capture_output=True, text=True, encoding="utf-8", check=False)
    assert direct.returncode != 0
    assert via_package.returncode == direct.returncode
    assert via_package.stderr == direct.stderr
    version = python(TFG, "version")
    assert version.returncode == 0
    assert version.stdout == subprocess.run([tfg_binary(), "version"], capture_output=True, text=True,
                                            encoding="utf-8", check=False).stdout


def test_a_package_with_no_binary_says_so_with_the_code_of_a_tool_that_could_not_run(tmp_path):
    gone = python(TFG, "version", env={"TFG_BINARY": str(tmp_path / "gone")})
    assert gone.returncode == 1
    assert gone.stderr.startswith("tfg: TFG_BINARY is set to")
