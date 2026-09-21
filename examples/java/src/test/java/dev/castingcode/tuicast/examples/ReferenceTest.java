package dev.castingcode.tuicast.examples;

import static org.junit.jupiter.api.Assertions.*;

import dev.castingcode.tuicast.*;
import java.time.Duration;
import java.util.ArrayList;
import java.util.concurrent.Executors;
import org.junit.jupiter.api.*;

class ReferenceTest {
  Driver driver;
  Connection connection;
  final ArrayList<Session> sessions = new ArrayList<>();

  @BeforeEach
  void connect() {
    driver =
        Driver.launch(
            new Driver.Options(
                env("TUICAST_DRIVER", "tuicast-driver"), null, null, Duration.ofSeconds(30), null));
    connection =
        driver.connect(
            SshConfig.builder(env("TUICAST_REFERENCE_ADDRESS", "127.0.0.1:2222"), "operator")
                .password("casting")
                .insecureSkipHostKeyCheck(true)
                .build());
  }

  @AfterEach
  void close() {
    sessions.forEach(Session::close);
    if (connection != null) connection.close();
    if (driver != null) driver.close();
  }

  Session session() {
    var value =
        connection.openSession(new SessionConfig(TerminalProfile.XTERM_256_COLOR, 80, 24, ""));
    sessions.add(value);
    login(value);
    return value;
  }

  void login(Session s) {
    s.waitForText("LOGIN / AUTHENTICATION", Duration.ofSeconds(30), Duration.ofMillis(50));
    s.type("operator");
    s.press(Key.Tab);
    s.type("casting");
    s.press(Key.Enter);
    assertTrue(s.waitForText("TERMINAL TEST SYSTEM").contains("Authenticated as operator"));
  }

  Screen open(Session s, int index, String heading) {
    for (int i = 0; i < index; i++) s.press(Key.ArrowDown);
    s.press(Key.Enter);
    return s.waitForText(heading, Duration.ofSeconds(30), Duration.ofMillis(50));
  }

  @Test
  void form() {
    var s = session();
    open(s, 1, "RECEIVING FORM");
    s.press(Key.Tab, Modifier.Shift);
    s.press(Key.Tab, Modifier.Shift);
    s.press(Key.Enter);
    assertTrue(
        s.waitFor(
                Matcher.all(
                    Matcher.contains("purchase order is required"),
                    Matcher.contains("quantity must be a positive number")))
            .contains("required"));
    for (String value : new String[] {"PO-10002341", "WIDGET-42", "25", "A-01-02", "dock 3"}) {
      s.type(value);
      s.press(Key.Tab);
    }
    s.press(Key.ArrowRight);
    s.press(Key.Tab);
    s.press(Key.Enter);
    assertTrue(
        s.waitForText("RECEIPT ACCEPTED")
            .contains("PO-10002341 / WIDGET-42 / quantity 25 / A-01-02 / Urgent"));
  }

  @Test
  void table() {
    var s = session();
    open(s, 2, "WAREHOUSE ORDERS");
    s.press('/');
    s.type("HOLD");
    s.press(Key.Enter);
    s.press(Key.End);
    s.press(Key.Enter);
    assertTrue(
        s.waitFor(
                Matcher.all(Matcher.contains("ORDER DETAILS"), Matcher.contains("Status:    HOLD")))
            .contains("Location:"));
  }

  @Test
  void concurrency() throws Exception {
    try (var executor = Executors.newVirtualThreadPerTaskExecutor()) {
      var futures =
          java.util.List.of(1, 2, 6).stream()
              .map(
                  i ->
                      executor.submit(
                          () ->
                              open(
                                  session(),
                                  i,
                                  i == 1
                                      ? "RECEIVING FORM"
                                      : i == 2 ? "WAREHOUSE ORDERS" : "FUNCTION KEYS")))
              .toList();
      for (var future : futures) assertNotNull(future.get());
    }
  }

  @Test
  void subscriptionsAndEvents() {
    var s = session();
    try (var screens = s.subscribe()) {
      open(s, 1, "RECEIVING FORM");
      assertTrue(screens.take(Duration.ofSeconds(1)).revision() > 0);
    }
    var e =
        connection.openSession(new SessionConfig(TerminalProfile.VT220, 80, 24, "TUICAST-ANSWER"));
    sessions.add(e);
    login(e);
    open(e, 11, "BELL / ENQ ANSWERBACK");
    try (var events = e.subscribeEvents()) {
      e.press('b');
      assertEquals("bell", events.take(Duration.ofSeconds(1)).type());
      e.press('e');
      assertEquals("TUICAST-ANSWER", events.take(Duration.ofSeconds(1)).data());
    }
  }

  @Test
  void synchronizationAndFiniteOperation() {
    var s = session();
    open(s, 7, "PARTIAL SCREEN UPDATES");
    s.press('1');
    s.waitForText("READY");
    assertTrue(
        s.waitFor(
                Matcher.all(
                    Matcher.contains("SCREEN COMPLETE / INPUT ENABLED"),
                    Matcher.contains("Inventory: VERIFIED")),
                Duration.ofSeconds(3),
                Duration.ofMillis(100))
            .contains("VERIFIED"));
    s.press('x');
    assertTrue(s.waitForText("Accepted input: x").contains("x"));
  }

  @Test
  void longRunningOperation() {
    var s = session();
    open(s, 8, "LONG-RUNNING OPERATION");
    s.press('1');
    var screen =
        s.waitFor(
            Matcher.all(
                Matcher.contains("Processed: 120"),
                Matcher.contains("Status: OPERATION COMPLETE"),
                Matcher.not(Matcher.contains("STREAMING"))),
            Duration.ofSeconds(12),
            Duration.ZERO);
    assertTrue(screen.contains("Remaining: 0"));
  }

  @Test
  void waitDiagnostics() {
    var raw = connection.openSession();
    sessions.add(raw);
    var error =
        assertThrows(
            WaitException.class,
            () ->
                raw.waitForText(
                    "TEXT THAT WILL NOT APPEAR", Duration.ofMillis(100), Duration.ZERO));
    assertTrue(error.isTimeout());
    assertTrue(error.expected().contains("TEXT THAT WILL NOT APPEAR"));
    assertTrue(error.lastScreen().contains("LOGIN / AUTHENTICATION"));
  }

  @Test
  void resize() {
    var s = session();
    open(s, 9, "TERMINAL RESIZE");
    s.type("value survives");
    s.press(Key.ArrowDown);
    s.press('T');
    s.resize(132, 24);
    var screen =
        s.waitFor(
            Matcher.all(
                Matcher.contains("Actual dimensions: 132x24"), Matcher.contains("value survives")));
    assertEquals(132, screen.width());
  }

  @Test
  void structuredInspectionAndKeys() {
    var s = session();
    var colors = open(s, 4, "ANSI COLORS AND ATTRIBUTES");
    var p = colors.find("BOLD").orElseThrow();
    assertTrue(colors.cellAt(p.column(), p.row()).orElseThrow().has(Screen.BOLD));
    var u = session();
    var unicode = open(u, 10, "UNICODE ALIGNMENT");
    var q = unicode.find("漢字").orElseThrow();
    assertEquals(2, unicode.cellAt(q.column(), q.row()).orElseThrow().width());
    assertEquals(0, unicode.cellAt(q.column() + 1, q.row()).orElseThrow().width());
    var k = session();
    open(k, 6, "FUNCTION KEYS");
    k.press(Key.F5);
    k.waitForText("Last: f5");
    k.press('c', Modifier.Control);
    assertTrue(k.waitForText("Last: ctrl+c").contains("Modifiers: control"));
  }

  @Test
  void telnet() {
    connection.close();
    driver.close();
    driver =
        Driver.launch(
            new Driver.Options(
                env("TUICAST_DRIVER", "tuicast-driver"), null, null, Duration.ofSeconds(30), null));
    connection =
        driver.connect(new TelnetConfig(env("TUICAST_REFERENCE_TELNET_ADDRESS", "127.0.0.1:2323")));
    assertTrue(open(session(), 1, "RECEIVING FORM").contains("Purchase order"));
  }

  private static String env(String name, String fallback) {
    return System.getenv().getOrDefault(name, fallback);
  }
}
