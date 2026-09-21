import html

import tuicast

ANSI = [
    "#000000",
    "#cd3131",
    "#0dbc79",
    "#e5e510",
    "#2472c8",
    "#bc3fbc",
    "#11a8cd",
    "#e5e5e5",
    "#666666",
    "#f14c4c",
    "#23d18b",
    "#f5f543",
    "#3b8eea",
    "#d670d6",
    "#29b8db",
    "#ffffff",
]


def color(index, fallback):
    if index < 0:
        return fallback
    if index < 16:
        return ANSI[index]
    if index <= 231:
        n = index - 16
        levels = (0, 95, 135, 175, 215, 255)
        return f"#{levels[n // 36]:02x}{levels[n // 6 % 6]:02x}{levels[n % 6]:02x}"
    if index <= 255:
        n = 8 + (index - 232) * 10
        return f"#{n:02x}{n:02x}{n:02x}"
    return fallback


def terminal_capture_html(screen):
    view_width = screen.width * 9 + 24
    view_height = screen.height * 18 + 24
    parts = [
        f"<div>Terminal {screen.width}x{screen.height} · revision {screen.revision}</div>",
        '<svg xmlns="http://www.w3.org/2000/svg" '
        f'viewBox="0 0 {view_width} {view_height}" style="background:#1e1e2e">',
    ]
    for row in range(screen.height):
        for column in range(screen.width):
            cell = screen.cell_at(column, row)
            if not cell or not cell.width:
                continue
            fg, bg = (
                color(cell.foreground, "#d8dee9"),
                color(cell.background, "#1e1e2e"),
            )
            if cell.attributes & tuicast.Attributes.REVERSE:
                fg, bg = bg, fg
            x, y = 12 + column * 9, 12 + row * 18
            if bg != "#1e1e2e":
                parts.append(
                    f'<rect x="{x}" y="{y}" width="{cell.width * 9}" height="18" fill="{bg}"/>'
                )
            if cell.text.strip() and not cell.attributes & tuicast.Attributes.CONCEAL:
                weight = "bold" if cell.attributes & tuicast.Attributes.BOLD else "normal"
                decoration = (
                    "underline" if cell.attributes & tuicast.Attributes.UNDERLINE else "none"
                )
                parts.append(
                    f'<text x="{x}" y="{y + 14}" fill="{fg}" '
                    'font-family="monospace" font-size="14" '
                    f'font-weight="{weight}" text-decoration="{decoration}">'
                    f"{html.escape(cell.text)}</text>"
                )
    if screen.cursor.visible:
        cursor_x = 12 + screen.cursor.column * 9
        cursor_y = 12 + screen.cursor.row * 18
        cursor = f'<rect x="{cursor_x}" y="{cursor_y}" width="9" height="18" '
        cursor += 'fill="none" stroke="#fff"/>'
        parts.append(cursor)
    return "".join(parts) + "</svg>"
