package dev.castingcode.tuicast;

public class TuicastException extends RuntimeException {
  public TuicastException(String message) {
    super(message);
  }

  public TuicastException(String message, Throwable cause) {
    super(message, cause);
  }
}
