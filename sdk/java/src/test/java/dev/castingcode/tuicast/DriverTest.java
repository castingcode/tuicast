package dev.castingcode.tuicast;

import static org.junit.jupiter.api.Assertions.*;

import java.io.OutputStream;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Map;
import java.util.concurrent.Executors;
import org.junit.jupiter.api.Test;

class DriverTest {
  private Driver launch(String mode) {
    String javaExecutable = Path.of(System.getProperty("java.home"), "bin", "java").toString();
    return Driver.launch(
        new Driver.Options(
            javaExecutable,
            java.util.List.of(
                "-cp", System.getProperty("java.class.path"), HelperDriver.class.getName(), mode),
            Map.of(),
            Duration.ofSeconds(2),
            OutputStream.nullOutputStream()));
  }

  @Test
  void lifecycleAndOperations() throws Exception {
    try (var driver = launch("normal");
        var connection = driver.connect(new TelnetConfig("host:23"));
        var session = connection.openSession()) {
      assertEquals(11, connection.id());
      assertEquals(22, session.id());
      session.type("operator");
      session.send(new byte[] {0, 1});
      session.press(Key.Enter);
      session.press('c', Modifier.Control);
      session.resize(132, 24);
      assertEquals("READY", session.screen().text());
      assertTrue(session.waitForText("READY").contains("READY"));
      assertTrue(session.waitForTextGone("LOADING").contains("READY"));
      assertEquals(3, session.waitForIdle().revision());
      WaitException failure =
          assertThrows(WaitException.class, () -> session.waitForText("MISSING"));
      assertTrue(failure.isTimeout());
      assertEquals("timeout", failure.kind());
      assertTrue(failure.expected().contains("MISSING"));
      assertEquals("READY", failure.lastScreen().text());
      assertEquals(-32000, failure.code());
      assertNotNull(failure.data());
      session.close();
      session.close();
      connection.close();
      connection.close();
    }
  }

  @Test
  void concurrentCallsAreCorrelated() throws Exception {
    try (var driver = launch("normal");
        var connection = driver.connect(new TelnetConfig("host:23"));
        var session = connection.openSession();
        var executor = Executors.newVirtualThreadPerTaskExecutor()) {
      var calls = new ArrayList<java.util.concurrent.Future<Screen>>();
      for (int i = 0; i < 30; i++) calls.add(executor.submit(session::screen));
      for (var call : calls) assertEquals("READY", call.get().text());
    }
  }

  @Test
  void subscriptionsPreserveTheirDeliveryContracts() throws Exception {
    try (var driver = launch("normal");
        var connection = driver.connect(new TelnetConfig("host:23"));
        var session = connection.openSession()) {
      try (var screens = session.subscribe()) {
        Thread.sleep(50);
        assertEquals(3, screens.take(Duration.ofSeconds(1)).revision());
        assertNull(screens.take(Duration.ofMillis(10)));
      }
      try (var events = session.subscribeEvents()) {
        assertEquals("bell", events.take(Duration.ofSeconds(1)).type());
        assertEquals("enquiry", events.take(Duration.ofSeconds(1)).type());
        events.close();
        events.close();
      }
    }
  }

  @Test
  void malformedProcessesAndProtocolAreRejected() {
    for (String mode :
        java.util.List.of("malformed", "wrong-version", "wrong-protocol", "exit", "no-result"))
      assertThrows(TuicastException.class, () -> launch(mode), mode);
    assertThrows(
        TuicastException.class,
        () ->
            Driver.launch(
                new Driver.Options(
                    "/definitely/missing", null, null, Duration.ofMillis(10), null)));
    assertThrows(
        IllegalArgumentException.class,
        () -> new Driver.Options(null, null, null, Duration.ZERO, null));
  }
}
