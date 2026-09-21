package dev.castingcode.tuicast;

public final class WaitException extends RpcException {
  private final String kind, expected;
  private final Screen lastScreen;

  WaitException(RpcException cause, String kind, String expected, Screen screen) {
    super(cause.code(), cause.getMessage(), cause.data());
    this.kind = kind;
    this.expected = expected;
    this.lastScreen = screen;
    initCause(cause);
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
