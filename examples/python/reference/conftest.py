import os

import pytest
import tuicast


@pytest.fixture
def fixture():
    driver = tuicast.Driver.launch(os.getenv("TUICAST_DRIVER", "tuicast-driver"))
    connection = driver.connect(
        tuicast.SSH(
            os.getenv("TUICAST_REFERENCE_ADDRESS", "127.0.0.1:2222"),
            "operator",
            password="casting",
            insecure_skip_host_key_check=True,
        )
    )
    yield connection
    connection.close()
    driver.close()


def login(session):
    session.wait_for_text("LOGIN / AUTHENTICATION", stable_for=0.05)
    session.type("operator")
    session.press(tuicast.Key.TAB)
    session.type("casting")
    session.press(tuicast.Key.ENTER)
    session.wait_for_text("TERMINAL TEST SYSTEM")


def open_scenario(session, index, heading):
    for _ in range(index):
        session.press(tuicast.Key.ARROW_DOWN)
    session.press(tuicast.Key.ENTER)
    return session.wait_for_text(heading, stable_for=0.1)
