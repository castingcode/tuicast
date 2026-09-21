# Python Driver Client

The independently publishable Python 3.12+ package is in `sdk/python` and is a
thin synchronous client for protocol v1 over a child `tuicast-driver` process.
It provides typed SSH/Telnet configuration and `Driver`, `Connection`, and
`Session` context managers. Driver-side waits preserve timing semantics;
detached immutable `Screen` values provide local queries.

## Mandatory development environment

All Python installs and commands **must** use the repository-local virtual
environment. Never install project packages or tooling globally. Do not commit
`.venv`. `pyproject.toml` is the sole source of package metadata, build settings,
dependencies, and formatter/linter/type/test configuration.

From the repository root, create it exactly as follows:

```sh
python3.12 -m venv sdk/python/.venv
sdk/python/.venv/bin/python -m pip install --upgrade pip
sdk/python/.venv/bin/python -m pip install -e 'sdk/python[dev]'
```

Run development and CI checks from the repository root. Every command names the
mandatory virtual environment explicitly and reads tool configuration from
`pyproject.toml`:

```sh
sdk/python/.venv/bin/python -m ruff format --check sdk/python examples/python
sdk/python/.venv/bin/python -m ruff check sdk/python examples/python
sdk/python/.venv/bin/python -m mypy --config-file sdk/python/pyproject.toml sdk/python/src
sdk/python/.venv/bin/python -m pytest -c sdk/python/pyproject.toml sdk/python/tests
sdk/python/.venv/bin/python -m build sdk/python
sdk/python/.venv/bin/python -m pytest -c sdk/python/pyproject.toml --no-cov -m reference examples/python/reference
sdk/python/.venv/bin/python examples/python/cucumber/run.py
sdk/python/.venv/bin/python examples/python/cucumber/report.py cucumber-report.json cucumber-report.html
```

The SDK test command measures line and branch coverage together and enforces an
85% minimum.

The last three commands require locally built `tuicast-driver` and
`reference-tui` services. Set `TUICAST_DRIVER`, `TUICAST_REFERENCE_ADDRESS`, and
`TUICAST_REFERENCE_TELNET_ADDRESS` to override defaults. Cucumber bindings read
`examples/features` directly; feature files must never be copied.

## Publishing

PyPI publication uses trusted publishing through
`.github/workflows/publish-python.yml`; it requires no repository API token.
Configure the PyPI publisher with project `tuicast`, owner `castingcode`,
repository `tuicast`, workflow `publish-python.yml`, and environment `pypi`.
Configure the matching GitHub environment with required reviewers before the
first release.

The package version in `pyproject.toml` must match a tag named
`sdk/python/v<version>`. For example, version `0.0.1` is published by pushing
`sdk/python/v0.0.1`. The workflow runs the complete CI suite, builds the wheel
and source distribution in a separate job, verifies the tag and package
versions match, and gives OIDC permission only to the environment-protected
publish job.
