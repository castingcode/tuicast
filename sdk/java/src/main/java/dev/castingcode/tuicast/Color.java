package dev.castingcode.tuicast;

/**
 * Palette indexes for {@link Screen.Cell} foreground and background colors. {@link #DEFAULT}
 * selects the terminal default.
 */
public final class Color {
  public static final int DEFAULT = -1,
      BLACK = 0,
      RED = 1,
      GREEN = 2,
      YELLOW = 3,
      BLUE = 4,
      MAGENTA = 5,
      CYAN = 6,
      WHITE = 7;

  private Color() {}
}
