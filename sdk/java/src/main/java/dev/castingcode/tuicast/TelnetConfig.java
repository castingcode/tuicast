package dev.castingcode.tuicast;

import java.time.Duration;
import java.util.*;

public record TelnetConfig(String address, Duration connectTimeout) implements ConnectionConfig {
  public TelnetConfig(String address) {
    this(address, null);
  }

  public Map<String, Object> parameters(Duration d) {
    validate();
    return Map.of(
        "protocol", "telnet", "address", address, "connectTimeoutMilliseconds", millis(timeout(d)));
  }

  public Duration timeout(Duration d) {
    return connectTimeout == null ? d : connectTimeout;
  }

  private void validate() {
    if (address == null || address.isBlank())
      throw new IllegalArgumentException("Telnet address is required");
    millis(timeout(Duration.ofSeconds(30)));
  }

  static long millis(Duration d) {
    if (d == null || d.isZero() || d.isNegative())
      throw new IllegalArgumentException("duration must be positive");
    return Math.max(1, (d.toNanos() + 999_999) / 1_000_000);
  }
}
