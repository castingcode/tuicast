import sys
from pathlib import Path

import tuicast

sys.path.insert(0, str(Path(__file__).resolve().parent))
from capture import terminal_capture_html


def before_scenario(context, scenario):
    context.driver = context.connection = context.session = None
    context.last_capture_revision = None


def after_step(context, step):
    if context.session is None or step.status.name == "skipped":
        return
    failed = step.status.name != "passed"
    if not failed and step.step_type != "then":
        return
    screen = context.session.screen()
    if not failed and context.last_capture_revision == screen.revision:
        return
    context.attach("text/html", terminal_capture_html(screen).encode())
    if failed:
        context.attach("text/plain", screen.text.encode())
    context.last_capture_revision = screen.revision


def after_scenario(context, scenario):
    try:
        if context.session:
            screen = context.session.screen()
            if not screen.contains("LOGIN / AUTHENTICATION"):
                if not screen.contains("TERMINAL TEST SYSTEM"):
                    context.session.press(tuicast.Key.ESCAPE)
                    context.session.wait_for_text("TERMINAL TEST SYSTEM")
                context.session.press(tuicast.Key.F2)
                context.session.wait_for_text("Signed out")
            context.session.close()
        if context.connection:
            context.connection.close()
    finally:
        if context.driver:
            context.driver.close()
