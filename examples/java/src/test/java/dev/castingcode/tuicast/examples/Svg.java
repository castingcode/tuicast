package dev.castingcode.tuicast.examples;

import dev.castingcode.tuicast.Screen;
import java.nio.charset.StandardCharsets;

final class Svg {
  private static final String[] ANSI = {
    "#000000", "#cd3131", "#0dbc79", "#e5e510", "#2472c8", "#bc3fbc", "#11a8cd", "#e5e5e5",
    "#666666", "#f14c4c", "#23d18b", "#f5f543", "#3b8eea", "#d670d6", "#29b8db", "#ffffff"
  };

  private Svg() {}

  static byte[] render(Screen screen) {
    var out =
        new StringBuilder("<svg xmlns=\"http://www.w3.org/2000/svg\" role=\"img\" viewBox=\"0 0 ")
            .append(screen.width() * 9 + 24)
            .append(' ')
            .append(screen.height() * 18 + 24)
            .append("\" style=\"background:#1e1e2e\">");
    for (int row = 0; row < screen.height(); row++)
      for (int column = 0; column < screen.width(); column++) {
        var optional = screen.cellAt(column, row);
        if (optional.isEmpty() || optional.get().width() == 0) continue;
        var cell = optional.get();
        String foreground = color(cell.foreground(), "#d8dee9");
        String background = color(cell.background(), "#1e1e2e");
        if (cell.has(Screen.REVERSE)) {
          String swap = foreground;
          foreground = background;
          background = swap;
        }
        int x = 12 + column * 9, y = 12 + row * 18;
        if (!background.equals("#1e1e2e"))
          out.append("<rect x=\"")
              .append(x)
              .append("\" y=\"")
              .append(y)
              .append("\" width=\"")
              .append(Math.max(1, cell.width()) * 9)
              .append("\" height=\"18\" fill=\"")
              .append(background)
              .append("\"/>");
        if (!cell.text().isBlank() && !cell.has(Screen.CONCEAL))
          out.append("<text x=\"")
              .append(x)
              .append("\" y=\"")
              .append(y + 14)
              .append("\" fill=\"")
              .append(foreground)
              .append("\" font-family=\"monospace\" font-size=\"14\" font-weight=\"")
              .append(cell.has(Screen.BOLD) ? "bold" : "normal")
              .append("\" text-decoration=\"")
              .append(cell.has(Screen.UNDERLINE) ? "underline" : "none")
              .append("\">")
              .append(escape(cell.text()))
              .append("</text>");
      }
    if (screen.cursor().visible())
      out.append("<rect x=\"")
          .append(12 + screen.cursor().column() * 9)
          .append("\" y=\"")
          .append(12 + screen.cursor().row() * 18)
          .append("\" width=\"9\" height=\"18\" fill=\"none\" stroke=\"#ffffff\"/>");
    return out.append("</svg>").toString().getBytes(StandardCharsets.UTF_8);
  }

  private static String color(int index, String fallback) {
    if (index < 0) return fallback;
    if (index < 16) return ANSI[index];
    if (index <= 231) {
      int value = index - 16;
      int[] levels = {0, 95, 135, 175, 215, 255};
      return String.format(
          "#%02x%02x%02x", levels[value / 36], levels[value / 6 % 6], levels[value % 6]);
    }
    if (index <= 255) {
      int value = 8 + (index - 232) * 10;
      return String.format("#%02x%02x%02x", value, value, value);
    }
    return fallback;
  }

  private static String escape(String value) {
    return value.replace("&", "&amp;").replace("<", "&lt;").replace(">", "&gt;");
  }
}
