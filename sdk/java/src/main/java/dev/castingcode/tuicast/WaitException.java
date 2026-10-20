package dev.castingcode.tuicast;

import java.util.Optional;

/**
 * A failed wait. {@link #dump()} is the driver's flight-recorder dump in JSON-lines trace format:
 * the most recent host output (up to 64 KiB) with timing and screen revisions, unsupported-sequence
 * counts, and the failure. Save it with a test report to diagnose the failure later. {@link
 * #dumpPath()} is the file holding the same dump when the driver runs with {@code -dump-dir}.
 */
public final class WaitException extends RpcException {
  private final String kind, expected, dump, dumpPath;
  private final Screen lastScreen;

  WaitException(
      RpcException cause,
      String kind,
      String expected,
      Screen screen,
      String dump,
      String dumpPath) {
    super(cause.code(), cause.getMessage(), cause.data());
    this.kind = kind;
    this.expected = expected;
    this.lastScreen = screen;
    this.dump = dump;
    this.dumpPath = dumpPath;
    initCause(cause);
  }

  public Optional<String> dump() {
    return Optional.ofNullable(dump);
  }

  public Optional<String> dumpPath() {
    return Optional.ofNullable(dumpPath);
  }

  public String kind() {
    return kind;
  }

  public String expected() {
    return expected;
  }

  public Screen lastScreen() {
    return lastScreen;
  }

  public boolean isTimeout() {
    return "timeout".equals(kind);
  }
}
