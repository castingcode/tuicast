from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from enum import IntEnum, IntFlag, StrEnum
from types import MappingProxyType
from typing import Any


class Color(IntEnum):
    DEFAULT = -1
    BLACK = 0
    RED = 1
    GREEN = 2
    YELLOW = 3
    BLUE = 4
    MAGENTA = 5
    CYAN = 6
    WHITE = 7


class Attributes(IntFlag):
    BOLD = 1
    UNDERLINE = 2
    BLINK = 4
    REVERSE = 8
    CONCEAL = 16


class Terminal(StrEnum):
    VT220 = "vt220"
    XTERM_256COLOR = "xterm-256color"


class Key(StrEnum):
    ENTER = "Enter"
    TAB = "Tab"
    BACKSPACE = "Backspace"
    ESCAPE = "Escape"
    ARROW_UP = "ArrowUp"
    ARROW_DOWN = "ArrowDown"
    ARROW_RIGHT = "ArrowRight"
    ARROW_LEFT = "ArrowLeft"
    HOME = "Home"
    END = "End"
    INSERT = "Insert"
    DELETE = "Delete"
    PAGE_UP = "PageUp"
    PAGE_DOWN = "PageDown"
    F1 = "F1"
    F2 = "F2"
    F3 = "F3"
    F4 = "F4"
    F5 = "F5"
    F6 = "F6"
    F7 = "F7"
    F8 = "F8"
    F9 = "F9"
    F10 = "F10"
    F11 = "F11"
    F12 = "F12"


class Modifier(StrEnum):
    SHIFT = "Shift"
    CONTROL = "Control"
    ALT = "Alt"
    META = "Meta"


@dataclass(frozen=True, slots=True)
class Cell:
    text: str
    width: int
    foreground: int
    background: int
    attributes: Attributes


@dataclass(frozen=True, slots=True)
class Cursor:
    column: int
    row: int
    visible: bool


@dataclass(frozen=True, slots=True)
class Position:
    column: int
    row: int


@dataclass(frozen=True, slots=True)
class Screen:
    width: int
    height: int
    cells: tuple[Cell, ...]
    cursor: Cursor
    revision: int
    text: str

    @classmethod
    def from_dict(cls, value: Mapping[str, Any]) -> Screen:
        return cls(
            width=int(value["width"]),
            height=int(value["height"]),
            cells=tuple(
                Cell(
                    str(c["text"]),
                    int(c["width"]),
                    int(c["foreground"]),
                    int(c["background"]),
                    Attributes(int(c["attributes"])),
                )
                for c in value["cells"]
            ),
            cursor=Cursor(**value["cursor"]),
            revision=int(value["revision"]),
            text=str(value["text"]),
        )

    def contains(self, text: str) -> bool:
        return text in self.text

    def cell_at(self, column: int, row: int) -> Cell | None:
        if not (0 <= column < self.width and 0 <= row < self.height):
            return None
        index = row * self.width + column
        return self.cells[index] if index < len(self.cells) else None

    def line(self, row: int) -> str:
        if not 0 <= row < self.height:
            return ""
        return "".join(
            c.text
            for column in range(self.width)
            if (c := self.cell_at(column, row)) is not None and c.width != 0
        )

    def find(self, text: str) -> Position | None:
        if text == "" and self.width and self.height:
            return Position(0, 0)
        for row in range(self.height):
            for column in range(self.width):
                cell = self.cell_at(column, row)
                if cell is None or cell.width == 0:
                    continue
                candidate = ""
                for current in range(column, self.width):
                    nxt = self.cell_at(current, row)
                    if nxt is None:
                        break
                    if nxt.width:
                        candidate += nxt.text
                    if candidate.startswith(text):
                        return Position(column, row)
                    if not text.startswith(candidate):
                        break
        return None


@dataclass(frozen=True, slots=True)
class TerminalEvent:
    sequence: int
    type: str
    data: str = ""


Matcher = Mapping[str, Any]


def _frozen(value: dict[str, Any]) -> Matcher:
    return MappingProxyType(value)


def contains(text: str) -> Matcher:
    return _frozen({"contains": text})


def line_equals(row: int, text: str) -> Matcher:
    if row < 0:
        raise ValueError("row must not be negative")
    return _frozen({"line": {"row": row, "text": text}})


def cursor_at(column: int, row: int) -> Matcher:
    if column < 0 or row < 0:
        raise ValueError("cursor coordinates must not be negative")
    return _frozen({"cursor": {"column": column, "row": row}})


def all_of(*matchers: Matcher) -> Matcher:
    if not matchers:
        raise ValueError("all_of requires at least one matcher")
    return _frozen({"all": list(matchers)})


def any_of(*matchers: Matcher) -> Matcher:
    if not matchers:
        raise ValueError("any_of requires at least one matcher")
    return _frozen({"any": list(matchers)})


def not_(matcher: Matcher) -> Matcher:
    return _frozen({"not": matcher})
