# pytest-testing-files-generator

The pytest plugin of [testing-files-generator](https://pypi.org/project/testing-files-generator/): real files of exact sizes and formats for your tests, and what the system under test should do with each one.

This package has no code of its own. It installs testing-files-generator of the same version, and pytest loads its plugin on its own. Installing either package gives the same thing:

```bash
pip install pytest-testing-files-generator
```

```python
import re

import pytest
from playwright.sync_api import expect
from testing_files_generator import file_set

uploads = file_set(preset="upload-validation", options={"limit": "2mb", "bulk": 0})


@pytest.mark.parametrize("file", uploads.params())
def test_upload(file, page):
    page.goto("/upload")
    page.get_by_label("File").set_input_files(file.path())
    page.get_by_role("button", name="Upload").click()
    status = page.get_by_role("status")
    expect(status).to_have_text(re.compile("Uploaded|Refused"))
    file.check("accept" if "Uploaded" in status.text_content() else "reject")
```

Everything else is in [the README of testing-files-generator](https://github.com/donislawdev/TestingFilesGenerator/tree/main/python/testing-files-generator#readme).

This package is under the Mozilla Public License 2.0, in LICENSE.
