package dev.castingcode.tuicast;

import static org.junit.jupiter.api.Assertions.*;

import java.time.Duration;
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
  void matchers() {
    assertNotNull(Matcher.all(Matcher.contains("x"), Matcher.cursorAt(0, 0)).value());
    assertThrows(IllegalArgumentException.class, Matcher::all);
  }
}
