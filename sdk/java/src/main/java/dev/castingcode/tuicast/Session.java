package dev.castingcode.tuicast;

import java.time.Duration;
import java.util.*;
import java.util.Base64;
import java.util.concurrent.atomic.AtomicBoolean;

public final class Session implements AutoCloseable {
  private final Driver driver;
  private final long id;
  private final AtomicBoolean closed = new AtomicBoolean();

  Session(Driver d, long i) {
    driver = d;
    id = i;
  }

  public long id() {
    return id;
  }

  public void type(String text) {
    call(
        "session.send", Map.of("sessionId", id, "text", text), Void.class, driver.defaultTimeout());
  }

  public void send(byte[] bytes) {
    call(
        "session.send",
        Map.of("sessionId", id, "base64", Base64.getEncoder().encodeToString(bytes)),
        Void.class,
        driver.defaultTimeout());
  }

  public void press(Key key, Modifier... modifiers) {
    press(key.name(), modifiers);
  }

  public void press(char key, Modifier... modifiers) {
    press(String.valueOf(key), modifiers);
  }

  private void press(String key, Modifier... modifiers) {
    call(
        "session.press",
        Map.of(
            "sessionId",
            id,
            "key",
            key,
            "modifiers",
            Arrays.stream(modifiers).map(Enum::name).toList()),
        Void.class,
        driver.defaultTimeout());
  }

  public void resize(int width, int height) {
    if (width <= 0 || height <= 0) throw new IllegalArgumentException("positive size required");
    call(
        "session.resize",
        Map.of("sessionId", id, "width", width, "height", height),
        Void.class,
        driver.defaultTimeout());
  }

  public Screen screen() {
    return call("session.screen", Map.of("sessionId", id), Screen.class, driver.defaultTimeout());
  }

  public Screen waitFor(Matcher matcher) {
    return waitFor(matcher, driver.defaultTimeout(), Duration.ZERO);
  }

  public Screen waitFor(Matcher matcher, Duration timeout, Duration stable) {
    Objects.requireNonNull(matcher);
    return call(
        "session.wait",
        Map.of(
            "sessionId",
            id,
            "matcher",
            matcher.value(),
            "timeoutMilliseconds",
            TelnetConfig.millis(timeout),
            "stableMilliseconds",
            stable.isZero() ? 0 : TelnetConfig.millis(stable)),
        Screen.class,
        timeout.plusSeconds(1));
  }

  public Screen waitForText(String text) {
    return waitFor(Matcher.contains(text));
  }

  public Screen waitForText(String text, Duration timeout, Duration stable) {
    return waitFor(Matcher.contains(text), timeout, stable);
  }

  public Screen waitForTextGone(String text) {
    return waitFor(Matcher.not(Matcher.contains(text)));
  }

  public Screen waitForIdle() {
    return waitForIdle(driver.defaultTimeout(), Duration.ofMillis(100));
  }

  public Screen waitForIdle(Duration timeout, Duration quiet) {
    return call(
        "session.waitForIdle",
        Map.of(
            "sessionId",
            id,
            "timeoutMilliseconds",
            TelnetConfig.millis(timeout),
            "quietMilliseconds",
            TelnetConfig.millis(quiet)),
        Screen.class,
        timeout.plusSeconds(1));
  }

  public Subscription<Screen> subscribe() {
    return driver.subscribe(id, "session.subscribe", 1);
  }

  public Subscription<TerminalEvent> subscribeEvents() {
    return driver.subscribe(id, "session.subscribeEvents", 64);
  }

  private <T> T call(String m, Object p, Class<T> t, Duration d) {
    return driver.call(m, p, t, d);
  }

  public void close() {
    if (closed.compareAndSet(false, true))
      call("session.close", Map.of("sessionId", id), Void.class, driver.defaultTimeout());
  }
}
