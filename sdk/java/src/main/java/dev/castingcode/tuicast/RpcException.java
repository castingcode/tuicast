package dev.castingcode.tuicast;

import com.fasterxml.jackson.databind.JsonNode;

public class RpcException extends TuicastException {
  private final int code;
  private final JsonNode data;

  public RpcException(int code, String message, JsonNode data) {
    super("driver error " + code + ": " + message);
    this.code = code;
    this.data = data;
  }

  public int code() {
    return code;
  }

  public JsonNode data() {
    return data;
  }
}
