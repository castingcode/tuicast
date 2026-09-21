package dev.castingcode.tuicast;

import com.fasterxml.jackson.databind.*;
import com.fasterxml.jackson.databind.node.*;
import java.io.*;
import java.time.Duration;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;

public final class Driver implements AutoCloseable {
  static final ObjectMapper JSON = new ObjectMapper();
  private final Process process;
  private final BufferedWriter input;
  private final Duration timeout;
  private final AtomicLong ids = new AtomicLong();
  private final ConcurrentMap<Long, CompletableFuture<JsonNode>> pending =
      new ConcurrentHashMap<>();
  private final ConcurrentMap<Long, Subscription<?>> subscriptions = new ConcurrentHashMap<>();
  private final ConcurrentMap<Long, List<JsonNode>> queued = new ConcurrentHashMap<>();
  private final Object writeLock = new Object();
  private final AtomicBoolean closed = new AtomicBoolean();
  private volatile Throwable failure;

  public record Options(
      String executable,
      List<String> arguments,
      Map<String, String> environment,
      Duration defaultTimeout,
      OutputStream stderr) {
    public Options {
      if (executable == null) executable = "tuicast-driver";
      arguments = arguments == null ? List.of() : List.copyOf(arguments);
      environment = environment == null ? Map.of() : Map.copyOf(environment);
      if (defaultTimeout == null) defaultTimeout = Duration.ofSeconds(30);
      if (defaultTimeout.isNegative() || defaultTimeout.isZero())
        throw new IllegalArgumentException("default timeout must be positive");
      if (stderr == null) stderr = System.err;
    }

    public static Options defaults() {
      return new Options(null, null, null, null, null);
    }
  }

  public static Driver launch() {
    return launch(Options.defaults());
  }

  public static Driver launch(Options o) {
    try {
      var command = new ArrayList<String>();
      command.add(o.executable());
      command.addAll(o.arguments());
      var pb = new ProcessBuilder(command);
      pb.environment().putAll(o.environment());
      var p = pb.start();
      Thread.ofVirtual()
          .start(
              () -> {
                try (var errors = p.getErrorStream()) {
                  errors.transferTo(o.stderr());
                } catch (IOException ignored) {
                }
              });
      var d = new Driver(p, o.defaultTimeout());
      var ping = d.call("driver.ping", Map.of(), JsonNode.class, o.defaultTimeout());
      if (!"1".equals(ping.path("protocolVersion").asText())) {
        d.close();
        throw new TuicastException(
            "client requires protocol version 1, driver reported "
                + ping.path("protocolVersion").asText());
      }
      return d;
    } catch (IOException e) {
      throw new TuicastException("starting TUICast driver", e);
    }
  }

  private Driver(Process p, Duration timeout) {
    process = p;
    this.timeout = timeout;
    input = new BufferedWriter(new OutputStreamWriter(p.getOutputStream()));
    Thread.ofVirtual().name("tuicast-rpc-reader").start(this::read);
  }

  Duration defaultTimeout() {
    return timeout;
  }

  boolean isAlive() {
    return process.isAlive();
  }

  private void read() {
    try (var r = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
      String line;
      while ((line = r.readLine()) != null) {
        JsonNode n = JSON.readTree(line);
        if (!"2.0".equals(n.path("jsonrpc").asText()))
          throw new IOException("unsupported JSON-RPC version");
        if (n.has("method")) {
          notify(n);
          continue;
        }
        long id = n.path("id").asLong();
        var future = pending.remove(id);
        if (future == null) continue;
        if (n.has("error")) {
          var e = n.get("error");
          future.completeExceptionally(error(e));
        } else if (!n.has("result"))
          future.completeExceptionally(new IOException("response has no result"));
        else future.complete(n.get("result"));
      }
    } catch (Throwable e) {
      fail(e);
    }
  }

  private RuntimeException error(JsonNode e) {
    var rpc = new RpcException(e.path("code").asInt(), e.path("message").asText(), e.get("data"));
    var d = e.get("data");
    if (d != null && d.has("kind")) {
      try {
        return new WaitException(
            rpc,
            d.path("kind").asText(),
            d.path("expected").asText(),
            JSON.treeToValue(d.get("screen"), Screen.class));
      } catch (Exception ignored) {
      }
    }
    return rpc;
  }

  private void notify(JsonNode n) {
    var p = n.get("params");
    if (p == null) return;
    long id = p.path("subscriptionId").asLong();
    var s = subscriptions.get(id);
    if (s == null) {
      queued.compute(
          id,
          (k, v) -> {
            var x = v == null ? new ArrayList<JsonNode>() : v;
            if ("session.screen".equals(n.path("method").asText())) x.clear();
            if (x.size() < 64) x.add(p.deepCopy());
            return x;
          });
      return;
    }
    deliver(s, n.path("method").asText(), p);
  }

  @SuppressWarnings("unchecked")
  private void deliver(Subscription<?> raw, String method, JsonNode p) {
    try {
      if (method.equals("session.screen"))
        ((Subscription<Screen>) raw).coalesce(JSON.treeToValue(p.get("screen"), Screen.class));
      else if (method.equals("session.event"))
        ((Subscription<TerminalEvent>) raw)
            .ordered(JSON.treeToValue(p.get("event"), TerminalEvent.class));
    } catch (Exception ignored) {
    }
  }

  private void fail(Throwable e) {
    failure = e;
    pending.forEach((k, v) -> v.completeExceptionally(e));
    pending.clear();
    subscriptions.values().forEach(Subscription::finish);
    subscriptions.clear();
  }

  <T> T call(String method, Object params, Class<T> type, Duration duration) {
    if (failure != null) throw new TuicastException("driver unavailable", failure);
    long id = ids.incrementAndGet();
    var f = new CompletableFuture<JsonNode>();
    pending.put(id, f);
    try {
      synchronized (writeLock) {
        input.write(
            JSON.writeValueAsString(
                Map.of("jsonrpc", "2.0", "id", id, "method", method, "params", params)));
        input.newLine();
        input.flush();
      }
      var result = f.get(duration.toMillis(), TimeUnit.MILLISECONDS);
      return type == Void.class ? null : JSON.treeToValue(result, type);
    } catch (TimeoutException e) {
      pending.remove(id);
      throw new TuicastException("calling " + method + ": timed out", e);
    } catch (ExecutionException e) {
      if (e.getCause() instanceof RuntimeException r) throw r;
      throw new TuicastException("calling " + method, e.getCause());
    } catch (Exception e) {
      throw new TuicastException("calling " + method, e);
    }
  }

  public Connection connect(ConnectionConfig config) {
    Objects.requireNonNull(config);
    var result =
        call(
            "connection.open",
            config.parameters(timeout),
            JsonNode.class,
            config.timeout(timeout).plusSeconds(1));
    return new Connection(this, result.path("connectionId").asLong());
  }

  <T> Subscription<T> subscribe(long session, String method, int capacity) {
    var r = call(method, Map.of("sessionId", session), JsonNode.class, timeout);
    long id = r.path("subscriptionId").asLong();
    var s = new Subscription<T>(id, this, capacity);
    subscriptions.put(id, s);
    var q = queued.remove(id);
    if (q != null)
      q.forEach(
          p ->
              deliver(
                  s, method.equals("session.subscribe") ? "session.screen" : "session.event", p));
    return s;
  }

  void removeSubscription(long id) {
    subscriptions.remove(id);
    queued.remove(id);
  }

  public synchronized void close() {
    if (!closed.compareAndSet(false, true)) return;
    subscriptions.values().forEach(Subscription::close);
    if (process.isAlive())
      try {
        call("driver.shutdown", Map.of(), Void.class, timeout);
      } catch (RuntimeException ignored) {
      }
    try {
      input.close();
      if (!process.waitFor(timeout.toMillis(), TimeUnit.MILLISECONDS)) process.destroyForcibly();
    } catch (Exception e) {
      throw new TuicastException("closing driver", e);
    }
  }
}
