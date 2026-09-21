import json
import sys
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

import pytest

import tuicast

HELPER = Path(__file__).with_name("helper_driver.py")


@pytest.fixture
def driver():
    value = tuicast.Driver.launch(sys.executable, args=(str(HELPER),), default_timeout=1)
    yield value
    value.close()


def test_lifecycle_concurrency_waits_and_subscriptions(driver):
    connection = driver.connect(tuicast.Telnet("example:23"))
    session = connection.open_session()
    session.type("hello")
    session.send(b"\x00")
    session.press(tuicast.Key.ENTER)
    session.resize(132, 24)
    with ThreadPoolExecutor(max_workers=8) as pool:
        assert all(
            screen.text == "READY" for screen in pool.map(lambda _: session.screen(), range(16))
        )
    assert session.wait_for_text("READY", stable_for=0.01).contains("READY")
    assert session.wait_for_text_gone("LOADING").text == "READY"
    assert session.wait_for_idle(quiet_for=0.01).text == "READY"
    with pytest.raises(tuicast.WaitError) as caught:
        session.wait_for_text("MISSING")
    assert tuicast.is_timeout(caught.value) and caught.value.last_screen.text == "READY"
    screens = session.subscribe()
    time.sleep(0.05)
    assert screens.get(timeout=1).revision == 3
    screens.close()
    screens.close()
    events = session.subscribe_events()
    assert [events.get(timeout=1).type, events.get(timeout=1).type] == ["bell", "enquiry"]
    events.close()
    session.close()
    session.close()
    connection.close()
    connection.close()
    driver.close()
    driver.close()


def test_configs_validation_matchers_and_timeout(driver):
    with pytest.raises(ValueError):
        tuicast.Telnet("").params(1)
    with pytest.raises(ValueError):
        tuicast.SSH("", "user", password="p", insecure_skip_host_key_check=True).params(1)
    with pytest.raises(ValueError):
        tuicast.SSH("host", "user", insecure_skip_host_key_check=True).params(1)
    with pytest.raises(ValueError):
        tuicast.SSH("host", "user", password="p").params(1)
    with pytest.raises(ValueError):
        tuicast.SSH(
            "host", "user", password="p", known_hosts_file="x", insecure_skip_host_key_check=True
        ).params(1)
    params, _ = tuicast.SSH(
        "host", "user", private_key="key", host_key_fingerprint="SHA256:x"
    ).params(1)
    assert params["protocol"] == "ssh"
    with pytest.raises(tuicast.RPCTimeoutError):
        driver._call("hang", timeout=0.01)
    with pytest.raises(TypeError):
        driver.connect(object())
    assert tuicast.all_of(tuicast.contains("x"), tuicast.not_(tuicast.cursor_at(1, 2)))["all"]
    with pytest.raises(ValueError):
        tuicast.any_of()
    with pytest.raises(ValueError):
        tuicast.line_equals(-1, "x")
    with pytest.raises(ValueError):
        tuicast.cursor_at(-1, 0)
    assert tuicast.is_timeout(TimeoutError())
    assert not tuicast.is_timeout(ValueError())

    connection = driver.connect(tuicast.Telnet("example:23"))
    with pytest.raises(ValueError):
        connection.open_session(width=0)
    with pytest.raises(ValueError):
        connection.open_session(answerback="\n")
    session = connection.open_session()
    with pytest.raises(ValueError):
        session.press("not-a-key")
    with pytest.raises(ValueError):
        session.resize(0, 24)
    with pytest.raises(ValueError):
        session.wait_for_text("READY", stable_for=-1)
    with pytest.raises(ValueError):
        session.wait_for_idle(quiet_for=0)
    session.close()
    connection.close()


def test_shared_screen_query_fixtures():
    fixtures = json.loads(
        (Path(__file__).parents[3] / "schema/testdata/screen-queries.json").read_text()
    )
    for case in fixtures["cases"]:
        screen = tuicast.Screen.from_dict(case["screen"])
        assert screen.contains(case["contains"])
        assert screen.find(case["find"]) == tuicast.Position(**case["position"])
        assert screen.line(case["position"]["row"])
        assert screen.line(-1) == ""
        assert screen.find("MISSING") is None
        assert screen.find("") == tuicast.Position(0, 0)
        assert screen.cell_at(-1, 0) is None


@pytest.mark.parametrize("mode", ["wrong", "malformed"])
def test_protocol_rejection(tmp_path, mode):
    script = tmp_path / "bad.py"
    output = (
        '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2"}}'
        if mode == "wrong"
        else "not json"
    )
    script.write_text(f"import sys\nsys.stdin.readline()\nprint({output!r}, flush=True)\n")
    with pytest.raises(tuicast.TUICastError):
        tuicast.Driver.launch(sys.executable, args=(str(script),), default_timeout=0.2)
