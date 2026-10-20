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
/**
 * A failed wait. `dump` is the driver's flight-recorder dump in JSON-lines
 * trace format: the most recent host output (up to 64 KiB) with timing and
 * screen revisions, unsupported-sequence counts, and the failure. Save it with
 * a test report to diagnose the failure later. `dumpPath` is the file holding
 * the same dump when the driver runs with `-dump-dir`.
 */
export class WaitError extends Error {
  constructor(
    readonly kind: string,
    readonly expected: string,
    readonly lastScreen: Screen,
    readonly cause: RPCError,
    readonly dump?: string,
    readonly dumpPath?: string,
  ) {
    super(`waiting for ${expected}: ${kind}; last screen:\n${lastScreen.text}`);
    this.name = "WaitError";
  }
}
export const isTimeout = (error: unknown): boolean =>
  (error instanceof WaitError && error.kind === "timeout") ||
  (error instanceof DOMException && error.name === "TimeoutError");
