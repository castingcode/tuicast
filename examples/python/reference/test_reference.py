import os
from concurrent.futures import ThreadPoolExecutor

import pytest
from conftest import login, open_scenario

import tuicast

pytestmark = pytest.mark.reference


def test_form_workflow(fixture):
    with fixture.open_session(terminal=tuicast.Terminal.XTERM_256COLOR) as session:
        login(session)
        screen = open_scenario(session, 1, "RECEIVING FORM")
        assert screen.contains("Purchase order")
        session.press(tuicast.Key.TAB, tuicast.Modifier.SHIFT)
        session.press(tuicast.Key.TAB, tuicast.Modifier.SHIFT)
        session.press(tuicast.Key.ENTER)
        session.wait_for(
            tuicast.all_of(
                tuicast.contains("purchase order is required"),
                tuicast.contains("quantity must be a positive number"),
            )
        )
        for value in ("PO-10002341", "WIDGET-42", "25", "A-01-02", "dock 3"):
            session.type(value)
            session.press(tuicast.Key.TAB)
        session.press(tuicast.Key.ARROW_RIGHT)
        session.press(tuicast.Key.TAB)
        session.press(tuicast.Key.ENTER)
        screen = session.wait_for_text("RECEIPT ACCEPTED")
        assert screen.contains("PO-10002341 / WIDGET-42 / quantity 25 / A-01-02 / Urgent")


def test_table_workflow(fixture):
    with fixture.open_session() as session:
        login(session)
        open_scenario(session, 2, "WAREHOUSE ORDERS")
        session.press("/")
        session.type("HOLD")
        session.press(tuicast.Key.ENTER)
        session.press(tuicast.Key.END)
        session.press(tuicast.Key.ENTER)
        screen = session.wait_for(
            tuicast.all_of(tuicast.contains("ORDER DETAILS"), tuicast.contains("Status:    HOLD"))
        )
        assert screen.contains("Location:")


def test_concurrent_sessions(fixture):
    def run(value):
        with fixture.open_session() as session:
            login(session)
            return open_scenario(session, *value).contains(value[1])

    scenarios = [(1, "RECEIVING FORM"), (2, "WAREHOUSE ORDERS"), (6, "FUNCTION KEYS")]
    with ThreadPoolExecutor(max_workers=3) as pool:
        assert all(pool.map(run, scenarios))


def test_screen_subscription(fixture):
    with fixture.open_session() as session:
        login(session)
        with session.subscribe() as screens:
            open_scenario(session, 1, "RECEIVING FORM")
            observed = screens.get(timeout=1)
            while not observed.contains("RECEIVING FORM"):
                observed = screens.get(timeout=1)
            assert observed.revision > 0


def test_terminal_event_subscription(fixture):
    with fixture.open_session(answerback="TUICAST-ANSWER") as session:
        login(session)
        open_scenario(session, 11, "BELL / ENQ ANSWERBACK")
        with session.subscribe_events() as events:
            session.press("b")
            assert events.get(timeout=1) == tuicast.TerminalEvent(1, "bell")
            session.press("e")
            assert events.get(timeout=1) == tuicast.TerminalEvent(2, "enquiry", "TUICAST-ANSWER")
        screen = session.wait_for_text('Answerback: "TUICAST-ANSWER"')
        assert screen.contains("BEL emitted: 1 / 1")
        assert screen.contains("ENQ emitted: 1 / 1")


def test_partial_screen_synchronization(fixture):
    with fixture.open_session() as session:
        login(session)
        open_scenario(session, 7, "PARTIAL SCREEN UPDATES")
        session.press("1")
        session.wait_for_text("READY")
        screen = session.wait_for(
            tuicast.all_of(
                tuicast.contains("READY"),
                tuicast.contains("SCREEN COMPLETE / INPUT ENABLED"),
            ),
            stable_for=0.1,
        )
        assert screen.contains("Inventory: VERIFIED")
        session.wait_for_idle(quiet_for=0.1)
        session.press("x")
        session.wait_for_text("Accepted input: x")


def test_long_running_operation(fixture):
    with fixture.open_session() as session:
        login(session)
        open_scenario(session, 8, "LONG-RUNNING OPERATION")
        session.press("1")
        screen = session.wait_for(
            tuicast.all_of(
                tuicast.contains("Processed: 120"),
                tuicast.contains("Status: OPERATION COMPLETE"),
                tuicast.not_(tuicast.contains("STREAMING")),
            ),
            timeout=12,
        )
        assert screen.contains("Remaining: 0")


def test_wait_diagnostics(fixture):
    with fixture.open_session() as session:
        with pytest.raises(tuicast.WaitError) as caught:
            session.wait_for_text("TEXT THAT WILL NOT APPEAR", timeout=0.1)
        assert tuicast.is_timeout(caught.value)
        assert "TEXT THAT WILL NOT APPEAR" in caught.value.expected
        assert caught.value.last_screen.contains("LOGIN / AUTHENTICATION")


def test_telnet_workflow():
    with tuicast.Driver.launch(os.getenv("TUICAST_DRIVER", "tuicast-driver")) as driver:
        config = tuicast.Telnet(os.getenv("TUICAST_REFERENCE_TELNET_ADDRESS", "127.0.0.1:2323"))
        with driver.connect(config) as connection, connection.open_session() as session:
            login(session)
            screen = open_scenario(session, 1, "RECEIVING FORM")
            assert screen.contains("Purchase order")


def test_terminal_resize_preserves_state(fixture):
    with fixture.open_session(width=80, height=24) as session:
        login(session)
        open_scenario(session, 9, "TERMINAL RESIZE")
        session.type("value survives")
        session.press(tuicast.Key.ARROW_DOWN)
        session.press("T")
        session.resize(132, 24)
        screen = session.wait_for(
            tuicast.all_of(
                tuicast.contains("Actual dimensions: 132x24"),
                tuicast.contains("Status: MATCH"),
                tuicast.contains("value survives"),
                tuicast.contains("> ORD-10002342"),
            )
        )
        assert screen.width == 132


def test_structured_colors_attributes_and_wide_unicode(fixture):
    with fixture.open_session(terminal=tuicast.Terminal.XTERM_256COLOR) as session:
        login(session)
        screen = open_scenario(session, 4, "ANSI COLORS AND ATTRIBUTES")
        bold = screen.find("BOLD")
        underline = screen.find("UNDERLINE")
        palette = screen.find("196")
        assert (
            bold is not None
            and screen.cell_at(bold.column, bold.row).attributes & tuicast.Attributes.BOLD
        )
        assert (
            underline is not None
            and screen.cell_at(underline.column, underline.row).attributes
            & tuicast.Attributes.UNDERLINE
        )
        assert palette is not None
        cell = screen.cell_at(palette.column, palette.row)
        assert cell is not None and (cell.foreground, cell.background) == (15, 196)
    with fixture.open_session(terminal=tuicast.Terminal.XTERM_256COLOR) as session:
        login(session)
        screen = open_scenario(session, 10, "UNICODE ALIGNMENT")
        position = screen.find("漢字")
        assert position is not None
        wide = screen.cell_at(position.column, position.row)
        continuation = screen.cell_at(position.column + 1, position.row)
        assert wide is not None and wide.width == 2
        assert continuation is not None and continuation.width == 0


def test_named_keys_and_modifiers(fixture):
    with fixture.open_session(terminal=tuicast.Terminal.XTERM_256COLOR) as session:
        login(session)
        open_scenario(session, 6, "FUNCTION KEYS")
        session.press(tuicast.Key.F5)
        session.wait_for_text("Last: f5")
        session.press("c", tuicast.Modifier.CONTROL)
        screen = session.wait_for_text("Last: ctrl+c")
        assert screen.contains("Modifiers: control")
