package dev.castingcode.tuicast;

import java.util.List;
import java.util.Optional;

public record Screen(
    int width, int height, List<Cell> cells, Cursor cursor, long revision, String text) {
  public Screen {
    cells = List.copyOf(cells);
  }

  public record Cell(String text, int width, int foreground, int background, int attributes) {
    public boolean has(int mask) {
      return (attributes & mask) == mask;
    }
  }

  public record Cursor(int column, int row, boolean visible) {}

  public record Position(int column, int row) {}

  public static final int BOLD = 1, UNDERLINE = 2, BLINK = 4, REVERSE = 8, CONCEAL = 16;

  public boolean contains(String value) {
    return text.contains(value);
  }

  public Optional<Cell> cellAt(int column, int row) {
    int i = row * width + column;
    return column < 0 || row < 0 || column >= width || row >= height || i >= cells.size()
        ? Optional.empty()
        : Optional.of(cells.get(i));
  }

  public String line(int row) {
    if (row < 0 || row >= height) return "";
    var b = new StringBuilder();
    for (int c = 0; c < width; c++)
      cellAt(c, row).filter(x -> x.width() != 0).ifPresent(x -> b.append(x.text()));
    return b.toString();
  }

  public Optional<Position> find(String value) {
    if (value.isEmpty() && width > 0 && height > 0) return Optional.of(new Position(0, 0));
    for (int r = 0; r < height; r++)
      for (int c = 0; c < width; c++) {
        if (cellAt(c, r).map(Cell::width).orElse(0) == 0) continue;
        var b = new StringBuilder();
        for (int x = c; x < width; x++) {
          var cell = cellAt(x, r);
          if (cell.isEmpty()) break;
          if (cell.get().width() != 0) b.append(cell.get().text());
          if (b.toString().startsWith(value)) return Optional.of(new Position(c, r));
          if (!value.startsWith(b.toString())) break;
        }
      }
    return Optional.empty();
  }
}
