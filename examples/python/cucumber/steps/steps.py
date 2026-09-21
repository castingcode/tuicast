import os

from behave import given, then, when

import tuicast

PASSWORD = "casting"


@given("I am connected to the reference TUI")
def connect(context):
    context.driver = tuicast.Driver.launch(os.getenv("TUICAST_DRIVER", "tuicast-driver"))
    context.connection = context.driver.connect(
        tuicast.SSH(
            os.getenv("TUICAST_REFERENCE_ADDRESS", "127.0.0.1:2222"),
            "operator",
            password=PASSWORD,
            insecure_skip_host_key_check=True,
        )
    )
    context.session = context.connection.open_session(terminal=tuicast.Terminal.XTERM_256COLOR)
    context.session.wait_for_text("LOGIN / AUTHENTICATION", stable_for=0.05)


@given("I am logged in")
def logged_in(context):
    enter_username(context, "operator")
    enter_password(context, PASSWORD)
    press(context, "Enter")
    screen(context, "main menu")


@when('I enter username "{username}"')
def enter_username(context, username):
    context.session.type(username)


@when('I enter password "{password}"')
def enter_password(context, password):
    context.session.press(tuicast.Key.TAB)
    context.session.type(password)


@when("I press {name}")
def press(context, name):
    key, mods = {
        "Enter": (tuicast.Key.ENTER, ()),
        "F5": (tuicast.Key.F5, ()),
        "Control+C": ("c", (tuicast.Modifier.CONTROL,)),
    }[name]
    context.session.press(key, *mods)


@when('I select the "{option}" menu option')
def select(context, option):
    for _ in range({"Colors and Attributes": 4, "Function Keys": 6, "Unicode": 10}[option]):
        context.session.press(tuicast.Key.ARROW_DOWN)
    context.session.press(tuicast.Key.ENTER)


@then('the application is on the "{name}" screen')
def screen(context, name):
    heading = {
        "login": "LOGIN / AUTHENTICATION",
        "main menu": "TERMINAL TEST SYSTEM",
        "Colors and Attributes": "ANSI COLORS AND ATTRIBUTES",
        "Function Keys": "FUNCTION KEYS",
        "Unicode": "UNICODE ALIGNMENT",
    }[name]
    context.session.wait_for_text(heading, stable_for=0.1)


@then('the screen contains "{text}"')
def contains(context, text):
    context.session.wait_for_text(text)


@then("the application reports that authentication failed")
def auth_failed(context):
    contains(context, "Invalid user ID or password")


@then('"{text}" begins at zero-based column {column:d} and row {row:d}')
def begins(context, text, column, row):
    assert context.session.screen().find(text) == tuicast.Position(column, row)


@then('"{text}" is rendered with the "{attribute}" attribute')
def attribute(context, text, attribute):
    value = {
        "bold": tuicast.Attributes.BOLD,
        "underline": tuicast.Attributes.UNDERLINE,
    }[attribute]
    current = context.session.screen()
    pos = current.find(text)
    assert current.cell_at(pos.column, pos.row).attributes & value


@then('the "{text}" color sample uses foreground {foreground:d} and background {background:d}')
def sample(context, text, foreground, background):
    current = context.session.screen()
    pos = current.find(text)
    cell = current.cell_at(pos.column, pos.row)
    assert (cell.foreground, cell.background) == (foreground, background)


@then('the "{label}" sample renders "{text}" at zero-based column {column:d} and row {row:d}')
def unicode_sample(context, label, text, column, row):
    assert context.session.screen().find(text) == tuicast.Position(column, row)


@then("the captured key count is {count:d}")
def count(context, count):
    contains(context, f"Captured count: {count}")


@then('the latest key is "{key}"')
def latest(context, key):
    contains(context, f"Last: {key}")


@then('the latest key modifiers are "{modifiers}"')
def modifiers(context, modifiers):
    contains(context, f"Modifiers: {modifiers}")
