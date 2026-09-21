package dev.castingcode.tuicast;

public enum TerminalProfile {
  VT220("vt220"),
  XTERM_256_COLOR("xterm-256color");
  private final String wire;

  TerminalProfile(String w) {
    wire = w;
  }

  String wire() {
    return wire;
  }
}
