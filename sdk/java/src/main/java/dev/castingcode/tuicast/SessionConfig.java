package dev.castingcode.tuicast;

public record SessionConfig(TerminalProfile terminal, int width, int height, String answerback) {
  public static SessionConfig defaults() {
    return new SessionConfig(TerminalProfile.VT220, 80, 24, "");
  }

  public SessionConfig {
    if (terminal == null || width <= 0 || height <= 0)
      throw new IllegalArgumentException("terminal and positive size required");
    if (answerback == null) answerback = "";
    if (answerback.length() > 20 || answerback.chars().anyMatch(c -> c < 32 || c > 126))
      throw new IllegalArgumentException("answerback must be at most 20 printable ASCII bytes");
  }
}
