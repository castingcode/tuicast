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
export interface Cell {
  readonly text: string;
  readonly width: number;
  readonly foreground: number;
  readonly background: number;
  readonly attributes: number;
}
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
export interface TerminalEvent {
  readonly sequence: number;
  readonly type: "bell" | "enquiry";
  readonly data?: string;
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
