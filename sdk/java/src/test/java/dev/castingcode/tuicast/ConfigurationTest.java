package dev.castingcode.tuicast;

import static org.junit.jupiter.api.Assertions.*;

import java.time.Duration;
import java.util.List;
import java.util.Map;
import org.junit.jupiter.api.Test;

class ConfigurationTest {
  @Test
  void validates() {
    assertThrows(
        IllegalArgumentException.class,
        () -> new TelnetConfig("").parameters(Duration.ofSeconds(1)));
    assertThrows(
        IllegalArgumentException.class,
        () ->
            SshConfig.builder("host:22", "user")
                .password("x")
                .build()
                .parameters(Duration.ofSeconds(1)));
    assertDoesNotThrow(
        () ->
            SshConfig.builder("host:22", "user")
                .password("x")
                .insecureSkipHostKeyCheck(true)
                .build()
                .parameters(Duration.ofSeconds(1)));
    assertThrows(
        IllegalArgumentException.class,
        () -> new SessionConfig(TerminalProfile.VT220, 80, 24, "\n"));
  }

  @Test
  void ptyParameters() {
    assertThrows(
        IllegalArgumentException.class, () -> new PtyConfig("").parameters(Duration.ofSeconds(1)));
    var parameters =
        PtyConfig.builder("rf-client")
            .arguments("--warehouse", "DC1")
            .workingDirectory("/opt/wms")
            .environment("WMS_ENV", "test")
            .clearEnvironment(true)
            .connectTimeout(Duration.ofSeconds(2))
            .build()
            .parameters(Duration.ofSeconds(1));
    assertEquals(
        Map.of(
            "protocol",
            "pty",
            "command",
            "rf-client",
            "arguments",
            List.of("--warehouse", "DC1"),
            "workingDirectory",
            "/opt/wms",
            "environment",
            Map.of("WMS_ENV", "test"),
            "clearEnvironment",
            true,
            "connectTimeoutMilliseconds",
            2000L),
        parameters);
    assertEquals(
        Map.of(
            "protocol",
            "pty",
            "command",
            "rf-client",
            "arguments",
            List.of(),
            "environment",
            Map.of(),
            "clearEnvironment",
            false,
            "connectTimeoutMilliseconds",
            1000L),
        new PtyConfig("rf-client").parameters(Duration.ofSeconds(1)));
  }

  @Test
  void matchers() {
    assertNotNull(Matcher.all(Matcher.contains("x"), Matcher.cursorAt(0, 0)).value());
    assertThrows(IllegalArgumentException.class, Matcher::all);
  }
}
