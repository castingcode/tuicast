package dev.castingcode.tuicast;

import java.util.*;

public sealed interface Matcher permits Matcher.Spec {
  Object value();

  record Spec(Object value) implements Matcher {}

  static Matcher contains(String text) {
    return new Spec(Map.of("contains", text));
  }

  static Matcher lineEquals(int row, String text) {
    return new Spec(Map.of("line", Map.of("row", row, "text", text)));
  }

  static Matcher cursorAt(int column, int row) {
    return new Spec(Map.of("cursor", Map.of("column", column, "row", row)));
  }

  static Matcher all(Matcher... values) {
    return composite("all", values);
  }

  static Matcher any(Matcher... values) {
    return composite("any", values);
  }

  static Matcher not(Matcher value) {
    return new Spec(Map.of("not", Objects.requireNonNull(value).value()));
  }

  private static Matcher composite(String name, Matcher[] values) {
    if (values.length == 0) throw new IllegalArgumentException(name + " requires children");
    return new Spec(
        Map.of(name, Arrays.stream(values).map(x -> Objects.requireNonNull(x).value()).toList()));
  }
}
