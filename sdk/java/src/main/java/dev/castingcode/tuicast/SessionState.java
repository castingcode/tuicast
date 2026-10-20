package dev.castingcode.tuicast;

import java.util.Objects;
import java.util.Optional;

/**
 * Whether the host has ended a session. {@code error} describes the transport or terminal failure
 * that ended it and is empty while the session is active and after a normal end.
 */
public record SessionState(boolean ended, Optional<String> error) {
  public SessionState {
    Objects.requireNonNull(error);
  }
}
