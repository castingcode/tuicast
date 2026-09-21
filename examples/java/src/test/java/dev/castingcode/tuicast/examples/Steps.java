package dev.castingcode.tuicast.examples;

import static org.junit.jupiter.api.Assertions.*;

import dev.castingcode.tuicast.*;
import io.cucumber.java.*;
import io.cucumber.java.en.*;
import java.time.Duration;
import java.util.*;

public class Steps {
  Driver driver;
  Connection connection;
  Session session;
  Screen screen;
  boolean authenticated;
  Scenario scenario;
  long lastCaptureRevision = -1;

  @Before
  public void before(Scenario scenario) {
    this.scenario = scenario;
  }

  @Given("I am connected to the reference TUI")
  public void connect() {
    driver =
        Driver.launch(
            new Driver.Options(
                System.getenv().getOrDefault("TUICAST_DRIVER", "tuicast-driver"),
                List.of(),
                Map.of(),
                Duration.ofSeconds(30),
                System.err));
    connection =
        driver.connect(
            SshConfig.builder(
                    System.getenv().getOrDefault("TUICAST_REFERENCE_ADDRESS", "127.0.0.1:2222"),
                    "operator")
                .password("casting")
                .insecureSkipHostKeyCheck(true)
                .build());
    session =
        connection.openSession(new SessionConfig(TerminalProfile.XTERM_256_COLOR, 80, 24, ""));
    screen =
        session.waitForText(
            "LOGIN / AUTHENTICATION", Duration.ofSeconds(30), Duration.ofMillis(50));
  }

  @Given("I am logged in")
  public void login() {
    username("operator");
    password("casting");
    press("Enter");
    screen =
        session.waitForText("TERMINAL TEST SYSTEM", Duration.ofSeconds(30), Duration.ofMillis(50));
    authenticated = true;
  }

  @When("I enter username {string}")
  public void username(String s) {
    session.type(s);
  }

  @When("I enter password {string}")
  public void password(String s) {
    session.press(Key.Tab);
    session.type(s);
  }

  @When("I press {word}")
  public void press(String k) {
    switch (k) {
      case "Enter" -> session.press(Key.Enter);
      case "F5" -> session.press(Key.F5);
      case "Control+C" -> session.press('c', Modifier.Control);
      default -> throw new IllegalArgumentException(k);
    }
  }

  @When("I select the {string} menu option")
  public void menu(String name) {
    int n = Map.of("Colors and Attributes", 4, "Function Keys", 6, "Unicode", 10).get(name);
    for (int i = 0; i < n; i++) session.press(Key.ArrowDown);
    session.press(Key.Enter);
  }

  @Then("the application is on the {string} screen")
  public void onScreen(String name) {
    String heading =
        Map.of(
                "login",
                "LOGIN / AUTHENTICATION",
                "main menu",
                "TERMINAL TEST SYSTEM",
                "Colors and Attributes",
                "ANSI COLORS AND ATTRIBUTES",
                "Function Keys",
                "FUNCTION KEYS",
                "Unicode",
                "UNICODE ALIGNMENT")
            .get(name);
    screen = session.waitForText(heading, Duration.ofSeconds(30), Duration.ofMillis(50));
    if (name.equals("main menu")) authenticated = true;
    captureSuccess();
  }

  @Then("the screen contains {string}")
  public void contains(String s) {
    screen = session.waitForText(s);
    captureSuccess();
  }

  @Then("the application reports that authentication failed")
  public void failed() {
    contains("Invalid user ID or password");
  }

  @Then("{string} begins at zero-based column {int} and row {int}")
  public void position(String s, int c, int r) {
    assertEquals(new Screen.Position(c, r), session.screen().find(s).orElseThrow());
    captureSuccess();
  }

  @Then("{string} is rendered with the {string} attribute")
  public void attribute(String s, String a) {
    var sc = session.screen();
    var p = sc.find(s).orElseThrow();
    int mask = a.equals("bold") ? Screen.BOLD : Screen.UNDERLINE;
    assertTrue(sc.cellAt(p.column(), p.row()).orElseThrow().has(mask));
    captureSuccess();
  }

  @Then("the {string} color sample uses foreground {int} and background {int}")
  public void color(String s, int f, int b) {
    var sc = session.screen();
    var p = sc.find(s).orElseThrow();
    var c = sc.cellAt(p.column(), p.row()).orElseThrow();
    assertEquals(f, c.foreground());
    assertEquals(b, c.background());
    captureSuccess();
  }

  @Then("the {string} sample renders {string} at zero-based column {int} and row {int}")
  public void unicode(String label, String value, int c, int r) {
    position(value, c, r);
    assertEquals(r, session.screen().find(label).orElseThrow().row());
    captureSuccess();
  }

  @Then("the captured key count is {int}")
  public void count(int n) {
    contains("Captured count: " + n);
  }

  @Then("the latest key is {string}")
  public void latest(String s) {
    contains("Last: " + s);
  }

  @Then("the latest key modifiers are {string}")
  public void mods(String s) {
    contains("Modifiers: " + s);
  }

  private void captureSuccess() {
    var current = session.screen();
    if (current.revision() != lastCaptureRevision) {
      scenario.attach(
          Svg.render(current), "image/svg+xml", "terminal-revision-" + current.revision());
      lastCaptureRevision = current.revision();
    }
  }

  @AfterStep
  public void captureFailure(Scenario scenario) {
    if (!scenario.isFailed() || session == null) return;
    var current = session.screen();
    scenario.attach(Svg.render(current), "image/svg+xml", "failed-terminal");
    scenario.attach(current.text(), "text/plain", "failed-terminal");
  }

  @After
  public void close(Scenario scenario) {
    if (session != null) {
      if (authenticated) logout();
      try {
        session.close();
      } catch (Exception ignored) {
      }
    }
    if (connection != null)
      try {
        connection.close();
      } catch (Exception ignored) {
      }
    if (driver != null) driver.close();
  }

  private void logout() {
    try {
      var current = session.screen();
      if (!current.contains("TERMINAL TEST SYSTEM")) {
        session.press(Key.Escape);
        session.waitForText("TERMINAL TEST SYSTEM");
      }
      session.press(Key.F2);
      session.waitForText("Signed out");
    } catch (RuntimeException ignored) {
      // Cleanup is best effort and must not hide the scenario result.
    }
  }
}
