import { Screen } from "./screen.js";
export class RPCError extends Error {
  constructor(
    readonly code: number,
    message: string,
    readonly data?: unknown,
  ) {
    super(`driver error ${code}: ${message}`);
    this.name = "RPCError";
  }
}
export class WaitError extends Error {
  constructor(
    readonly kind: string,
    readonly expected: string,
    readonly lastScreen: Screen,
    readonly cause: RPCError,
  ) {
    super(`waiting for ${expected}: ${kind}; last screen:\n${lastScreen.text}`);
    this.name = "WaitError";
  }
}
export const isTimeout = (error: unknown): boolean =>
  (error instanceof WaitError && error.kind === "timeout") ||
  (error instanceof DOMException && error.name === "TimeoutError");
