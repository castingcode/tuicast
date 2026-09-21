package dev.castingcode.tuicast;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.util.LinkedHashMap;
import java.util.Map;

/** Deterministic JSON-RPC child process used by client tests. */
public final class HelperDriver {
  private static final ObjectMapper JSON = new ObjectMapper();

  private HelperDriver() {}

  public static void main(String[] args) throws Exception {
    String mode = args.length == 0 ? "normal" : args[0];
    if (mode.equals("malformed")) {
      System.out.println("not-json");
      return;
    }
    try (var reader = new BufferedReader(new InputStreamReader(System.in))) {
      String line;
      while ((line = reader.readLine()) != null) {
        JsonNode request = JSON.readTree(line);
        long id = request.path("id").asLong();
        String method = request.path("method").asText();
        if (mode.equals("wrong-version")) {
          write(Map.of("jsonrpc", "1.0", "id", id, "result", Map.of()));
          return;
        }
        if (mode.equals("exit")) return;
        Object result;
        switch (method) {
          case "driver.ping" ->
              result = Map.of("protocolVersion", mode.equals("wrong-protocol") ? "9" : "1");
          case "driver.shutdown" -> {
            write(ok(id, Map.of("shuttingDown", true)));
            return;
          }
          case "connection.open" -> result = Map.of("connectionId", 11);
          case "session.open" -> result = Map.of("sessionId", 22);
          case "connection.close",
                  "session.close",
                  "session.send",
                  "session.press",
                  "session.resize",
                  "session.unsubscribe" ->
              result = Map.of("ok", true);
          case "session.screen", "session.waitForIdle" -> result = screen(3);
          case "session.wait" -> {
            if (request
                .path("params")
                .path("matcher")
                .path("contains")
                .asText()
                .equals("MISSING")) {
              write(
                  Map.of(
                      "jsonrpc",
                      "2.0",
                      "id",
                      id,
                      "error",
                      Map.of(
                          "code",
                          -32000,
                          "message",
                          "wait timed out",
                          "data",
                          Map.of(
                              "kind",
                              "timeout",
                              "expected",
                              "screen containing MISSING",
                              "screen",
                              screen(3)))));
              continue;
            }
            result = screen(3);
          }
          case "session.subscribe", "session.subscribeEvents" -> {
            write(ok(id, Map.of("subscriptionId", 33)));
            if (method.endsWith("Events")) {
              notifyEvent(1, "bell", "");
              notifyEvent(2, "enquiry", "TUICAST");
            } else {
              notifyScreen(1);
              notifyScreen(2);
              notifyScreen(3);
            }
            continue;
          }
          default -> {
            write(
                Map.of(
                    "jsonrpc",
                    "2.0",
                    "id",
                    id,
                    "error",
                    Map.of("code", -32601, "message", "method not found")));
            continue;
          }
        }
        if (mode.equals("no-result")) write(Map.of("jsonrpc", "2.0", "id", id));
        else write(ok(id, result));
      }
    }
  }

  private static Map<String, Object> ok(long id, Object result) {
    return Map.of("jsonrpc", "2.0", "id", id, "result", result);
  }

  private static synchronized void write(Object value) throws Exception {
    System.out.println(JSON.writeValueAsString(value));
    System.out.flush();
  }

  private static void notifyScreen(int revision) throws Exception {
    write(
        Map.of(
            "jsonrpc",
            "2.0",
            "method",
            "session.screen",
            "params",
            Map.of("subscriptionId", 33, "screen", screen(revision))));
  }

  private static void notifyEvent(long sequence, String type, String data) throws Exception {
    write(
        Map.of(
            "jsonrpc",
            "2.0",
            "method",
            "session.event",
            "params",
            Map.of(
                "subscriptionId",
                33,
                "event",
                Map.of("sequence", sequence, "type", type, "data", data))));
  }

  private static Map<String, Object> screen(long revision) {
    var cells = new java.util.ArrayList<Map<String, Object>>();
    for (char value : "READY".toCharArray())
      cells.add(
          Map.of(
              "text",
              String.valueOf(value),
              "width",
              1,
              "foreground",
              -1,
              "background",
              -1,
              "attributes",
              0));
    var value = new LinkedHashMap<String, Object>();
    value.put("width", 5);
    value.put("height", 1);
    value.put("cells", cells);
    value.put("cursor", Map.of("column", 0, "row", 0, "visible", true));
    value.put("revision", revision);
    value.put("text", "READY");
    return value;
  }
}
