export type TerminalProfile = "vt220" | "xterm-256color";
export type Modifier = "Shift" | "Control" | "Alt" | "Meta";
export type NamedKey =
  | "Enter"
  | "Tab"
  | "Backspace"
  | "Escape"
  | "ArrowUp"
  | "ArrowDown"
  | "ArrowRight"
  | "ArrowLeft"
  | "Home"
  | "End"
  | "Insert"
  | "Delete"
  | "PageUp"
  | "PageDown"
  | `F${1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12}`;
export type Key = NamedKey | string;
export const Keys = {
  Enter: "Enter",
  Tab: "Tab",
  Backspace: "Backspace",
  Escape: "Escape",
  ArrowUp: "ArrowUp",
  ArrowDown: "ArrowDown",
  ArrowRight: "ArrowRight",
  ArrowLeft: "ArrowLeft",
  Home: "Home",
  End: "End",
  Insert: "Insert",
  Delete: "Delete",
  PageUp: "PageUp",
  PageDown: "PageDown",
  F1: "F1",
  F2: "F2",
  F3: "F3",
  F4: "F4",
  F5: "F5",
  F6: "F6",
  F7: "F7",
  F8: "F8",
  F9: "F9",
  F10: "F10",
  F11: "F11",
  F12: "F12",
} as const;
export const Attributes = {
  Bold: 1,
  Underline: 2,
  Blink: 4,
  Reverse: 8,
  Conceal: 16,
} as const;
/** Palette indexes for cell colors. `Default` selects the terminal default. */
export const Color = {
  Default: -1,
  Black: 0,
  Red: 1,
  Green: 2,
  Yellow: 3,
  Blue: 4,
  Magenta: 5,
  Cyan: 6,
  White: 7,
} as const;
export interface Cell {
  readonly text: string;
  readonly width: number;
  readonly foreground: number;
  readonly background: number;
  readonly attributes: number;
}
/** Reports whether a cell has all of the requested `Attributes` bits set. */
export const hasAttributes = (cell: Cell, attributes: number): boolean =>
  (cell.attributes & attributes) === attributes;
export interface Cursor {
  readonly column: number;
  readonly row: number;
  readonly visible: boolean;
}
export interface Position {
  readonly column: number;
  readonly row: number;
}
export interface ScreenData {
  width: number;
  height: number;
  cells: Cell[];
  cursor: Cursor;
  revision: number;
  text: string;
  title?: string;
}
export type Matcher =
  | { contains: string }
  | { line: { row: number; text: string } }
  | { cursor: { column: number; row: number } }
  | { all: Matcher[] }
  | { any: Matcher[] }
  | { not: Matcher };
export const contains = (text: string): Matcher => ({ contains: text });
export const lineEquals = (row: number, text: string): Matcher => ({
  line: { row, text },
});
export const cursorAt = (column: number, row: number): Matcher => ({
  cursor: { column, row },
});
export const all = (...children: Matcher[]): Matcher => ({ all: children });
export const any = (...children: Matcher[]): Matcher => ({ any: children });
export const not = (child: Matcher): Matcher => ({ not: child });
/**
 * A BELL, ENQ, or unsupported-sequence event. `data` is the configured
 * answerback for an enquiry and the ignored sequence, such as
 * `"CSI ? 2004 h"`, for an unsupported event.
 */
export interface TerminalEvent {
  readonly sequence: number;
  readonly type: "bell" | "enquiry" | "unsupported";
  readonly data?: string;
}
/**
 * Whether the host has ended a session. `error` describes the transport or
 * terminal failure that ended it and is absent after a normal end.
 */
export interface SessionState {
  readonly ended: boolean;
  readonly error?: string;
}
export interface TelnetConfig {
  protocol: "telnet";
  address: string;
  timeout?: number;
}
export interface SSHConfig {
  protocol: "ssh";
  address: string;
  username: string;
  password?: string;
  privateKey?: string;
  privateKeyPassphrase?: string;
  knownHostsFile?: string;
  hostKeyFingerprint?: string;
  insecureSkipHostKeyCheck?: boolean;
  timeout?: number;
}
export type ConnectionConfig = TelnetConfig | SSHConfig;
export interface SessionOptions {
  terminal?: TerminalProfile;
  width?: number;
  height?: number;
  answerback?: string;
  signal?: AbortSignal | undefined;
}
export interface OperationOptions {
  signal?: AbortSignal | undefined;
  timeout?: number | undefined;
}
export interface WaitOptions extends OperationOptions {
  stableFor?: number;
}
export interface IdleOptions extends OperationOptions {
  quietFor?: number;
}
