package dev.castingcode.tuicast;

import java.time.Duration;
import java.util.concurrent.*;

public final class Subscription<T> implements AutoCloseable {
  final long id;
  final Driver driver;
  final BlockingDeque<T> queue;
  private volatile boolean closed;

  Subscription(long id, Driver d, int capacity) {
    this.id = id;
    driver = d;
    queue = new LinkedBlockingDeque<>(capacity);
  }

  public T take(Duration timeout) {
    try {
      return queue.poll(timeout.toMillis(), TimeUnit.MILLISECONDS);
    } catch (InterruptedException e) {
      Thread.currentThread().interrupt();
      throw new TuicastException("waiting for subscription", e);
    }
  }

  void coalesce(T item) {
    queue.clear();
    queue.offer(item);
  }

  void ordered(T item) {
    if (!queue.offer(item)) close();
  }

  public boolean isClosed() {
    return closed;
  }

  public synchronized void close() {
    if (closed) return;
    closed = true;
    driver.removeSubscription(id);
    try {
      driver.call(
          "session.unsubscribe",
          java.util.Map.of("subscriptionId", id),
          Void.class,
          driver.defaultTimeout());
    } catch (RuntimeException ignored) {
      if (driver.isAlive()) throw ignored;
    }
  }

  void finish() {
    closed = true;
    queue.clear();
  }
}
