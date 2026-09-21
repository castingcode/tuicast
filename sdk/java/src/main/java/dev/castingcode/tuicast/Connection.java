package dev.castingcode.tuicast;

import com.fasterxml.jackson.databind.JsonNode;
import java.util.*;
import java.util.concurrent.atomic.AtomicBoolean;

public final class Connection implements AutoCloseable {
  private final Driver driver;
  private final long id;
  private final AtomicBoolean closed = new AtomicBoolean();

  Connection(Driver d, long i) {
    driver = d;
    id = i;
  }

  public long id() {
    return id;
  }

  public Session openSession() {
    return openSession(SessionConfig.defaults());
  }

  public Session openSession(SessionConfig c) {
    var r =
        driver.call(
            "session.open",
            Map.of(
                "connectionId",
                id,
                "terminal",
                c.terminal().wire(),
                "width",
                c.width(),
                "height",
                c.height(),
                "answerback",
                c.answerback()),
            JsonNode.class,
            driver.defaultTimeout());
    return new Session(driver, r.path("sessionId").asLong());
  }

  public void close() {
    if (closed.compareAndSet(false, true))
      driver.call(
          "connection.close", Map.of("connectionId", id), Void.class, driver.defaultTimeout());
  }
}
