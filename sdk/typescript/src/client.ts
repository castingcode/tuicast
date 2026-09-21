import { spawn, type ChildProcessWithoutNullStreams } from "node:child_process";
import { createInterface } from "node:readline";
import { RPCError, WaitError } from "./errors.js";
import { Screen } from "./screen.js";
import type {
  ConnectionConfig,
  IdleOptions,
  Key,
  Matcher,
  Modifier,
  OperationOptions,
  ScreenData,
  SessionOptions,
  TerminalEvent,
  WaitOptions,
} from "./types.js";

type Reply = {
  jsonrpc?: string;
  id?: number;
  method?: string;
  params?: any;
  result?: any;
  error?: { code: number; message: string; data?: any };
};
export interface LaunchOptions {
  path?: string;
  args?: string[];
  env?: NodeJS.ProcessEnv;
  stderr?: NodeJS.WritableStream;
  defaultTimeout?: number;
  signal?: AbortSignal;
}

class Queue<T> implements AsyncIterable<T> {
  private values: T[] = [];
  private waiters: Array<(x: IteratorResult<T>) => void> = [];
  private ended = false;
  constructor(
    private readonly capacity: number,
    private readonly coalesce = false,
  ) {}
  push(value: T) {
    if (this.ended) return;
    if (this.waiters.length) this.waiters.shift()!({ value, done: false });
    else if (this.coalesce) this.values = [value];
    else if (this.values.length < this.capacity) this.values.push(value);
    else this.end();
  }
  end() {
    if (this.ended) return;
    this.ended = true;
    for (const waiter of this.waiters.splice(0))
      waiter({ value: undefined, done: true });
  }
  [Symbol.asyncIterator]() {
    return {
      next: (): Promise<IteratorResult<T>> => {
        const value = this.values.shift();
        if (value !== undefined) return Promise.resolve({ value, done: false });
        if (this.ended)
          return Promise.resolve({ value: undefined, done: true });
        return new Promise((r) => this.waiters.push(r));
      },
    };
  }
}

abstract class Subscription<T> implements AsyncIterable<T> {
  protected readonly queue: Queue<T>;
  private closed = false;
  constructor(
    protected driver: Driver,
    readonly id: number,
    coalesce: boolean,
  ) {
    this.queue = new Queue<T>(coalesce ? 1 : 64, coalesce);
  }
  abstract deliver(params: any): void;
  finish() {
    this.queue.end();
  }
  [Symbol.asyncIterator]() {
    return this.queue[Symbol.asyncIterator]();
  }
  async close(options: OperationOptions = {}) {
    if (this.closed) return;
    this.closed = true;
    this.driver.removeSubscription(this.id);
    this.finish();
    await this.driver.call(
      "session.unsubscribe",
      { subscriptionId: this.id },
      options,
    );
  }
}
export class ScreenSubscription extends Subscription<Screen> {
  constructor(d: Driver, id: number) {
    super(d, id, true);
  }
  deliver(p: any) {
    this.queue.push(new Screen(p.screen as ScreenData));
  }
}
export class EventSubscription extends Subscription<TerminalEvent> {
  constructor(d: Driver, id: number) {
    super(d, id, false);
  }
  deliver(p: any) {
    this.queue.push(Object.freeze({ ...p.event }));
  }
}

export class Driver {
  private nextId = 0;
  private pending = new Map<
    number,
    { resolve: (v: any) => void; reject: (e: unknown) => void }
  >();
  private subscriptions = new Map<number, Subscription<any>>();
  private queued = new Map<number, Reply[]>();
  private failure?: Error;
  private closePromise?: Promise<void>;
  private constructor(
    private child: ChildProcessWithoutNullStreams,
    readonly defaultTimeout: number,
  ) {}
  static async launch(options: LaunchOptions = {}): Promise<Driver> {
    const timeout = options.defaultTimeout ?? 30_000;
    if (timeout <= 0) throw new TypeError("defaultTimeout must be positive");
    const child = spawn(options.path ?? "tuicast-driver", options.args ?? [], {
      stdio: ["pipe", "pipe", "pipe"],
      env: { ...process.env, ...options.env },
    });
    if (options.stderr) child.stderr.pipe(options.stderr);
    else child.stderr.pipe(process.stderr);
    const driver = new Driver(child, timeout);
    driver.listen();
    try {
      const ping = await driver.call("driver.ping", undefined, {
        signal: options.signal,
      });
      if (ping.protocolVersion !== "1")
        throw new Error(
          `client requires version 1, driver reported ${String(ping.protocolVersion)}`,
        );
      return driver;
    } catch (error) {
      child.kill();
      throw new Error("checking TUICast driver protocol", { cause: error });
    }
  }
  private listen() {
    createInterface({ input: this.child.stdout }).on("line", (line) => {
      let m: Reply;
      try {
        m = JSON.parse(line);
      } catch {
        this.fail(new Error("malformed driver response"));
        return;
      }
      if (m.jsonrpc !== "2.0") {
        this.fail(
          new Error(`unsupported JSON-RPC version ${String(m.jsonrpc)}`),
        );
        return;
      }
      if (m.id !== undefined) {
        const pending = this.pending.get(m.id);
        this.pending.delete(m.id);
        if (!pending) return;
        if (m.error) {
          const rpc = new RPCError(m.error.code, m.error.message, m.error.data);
          const d = m.error.data;
          pending.reject(
            d?.kind
              ? new WaitError(d.kind, d.expected, new Screen(d.screen), rpc)
              : rpc,
          );
        } else pending.resolve(m.result);
        return;
      }
      if (m.method === "session.screen" || m.method === "session.event") {
        const id = m.params?.subscriptionId as number;
        const sub = this.subscriptions.get(id);
        if (sub) sub.deliver(m.params);
        else {
          const q = this.queued.get(id) ?? [];
          this.queued.set(
            id,
            m.method === "session.screen" ? [m] : q.length < 64 ? [...q, m] : q,
          );
        }
      }
    });
    this.child.on("error", (e) =>
      this.fail(new Error("driver process failed", { cause: e })),
    );
    this.child.on("exit", () => this.fail(new Error("driver process exited")));
  }
  private fail(error: Error) {
    if (this.failure) return;
    this.failure = error;
    for (const p of this.pending.values()) p.reject(error);
    this.pending.clear();
    for (const s of this.subscriptions.values()) s.finish();
    this.subscriptions.clear();
  }
  async call(
    method: string,
    params?: unknown,
    options: OperationOptions = {},
  ): Promise<any> {
    if (this.failure) throw this.failure;
    const id = ++this.nextId;
    const timeout = options.timeout ?? this.defaultTimeout;
    const signal = options.signal
      ? AbortSignal.any([options.signal, AbortSignal.timeout(timeout)])
      : AbortSignal.timeout(timeout);
    return new Promise((resolve, reject) => {
      const abort = () => {
        this.pending.delete(id);
        reject(signal.reason);
      };
      if (signal.aborted) return abort();
      signal.addEventListener("abort", abort, { once: true });
      this.pending.set(id, {
        resolve: (v) => {
          signal.removeEventListener("abort", abort);
          resolve(v);
        },
        reject: (e) => {
          signal.removeEventListener("abort", abort);
          reject(e);
        },
      });
      this.child.stdin.write(
        JSON.stringify({
          jsonrpc: "2.0",
          id,
          method,
          ...(params === undefined ? {} : { params }),
        }) + "\n",
        (error) => {
          if (error) {
            this.pending.delete(id);
            reject(error);
          }
        },
      );
    });
  }
  async connect(
    config: ConnectionConfig,
    options: OperationOptions = {},
  ): Promise<Connection> {
    validateConnection(config);
    const timeout = config.timeout ?? this.defaultTimeout;
    const { timeout: _discard, ...rest } = config;
    const result = await this.call(
      "connection.open",
      { ...rest, connectTimeoutMilliseconds: timeout },
      { ...options, timeout: timeout + 1000 },
    );
    return new Connection(this, result.connectionId);
  }
  register(sub: Subscription<any>) {
    this.subscriptions.set(sub.id, sub);
    for (const n of this.queued.get(sub.id) ?? []) sub.deliver(n.params);
    this.queued.delete(sub.id);
  }
  removeSubscription(id: number) {
    this.subscriptions.delete(id);
  }
  close(): Promise<void> {
    return (this.closePromise ??= this.doClose());
  }
  private async doClose() {
    try {
      if (!this.failure)
        await this.call("driver.shutdown", undefined, {
          timeout: this.defaultTimeout,
        });
    } finally {
      this.child.stdin.end();
      if (this.child.exitCode === null)
        await new Promise<void>((resolve) => {
          const timer = setTimeout(() => {
            this.child.kill();
            resolve();
          }, this.defaultTimeout);
          this.child.once("exit", () => {
            clearTimeout(timer);
            resolve();
          });
        });
    }
  }
}

export class Connection {
  private closePromise?: Promise<void>;
  constructor(
    private driver: Driver,
    readonly id: number,
  ) {}
  async openSession(options: SessionOptions = {}): Promise<Session> {
    const terminal = options.terminal ?? "vt220",
      width = options.width ?? 80,
      height = options.height ?? 24,
      answerback = options.answerback ?? "";
    if (
      !["vt220", "xterm-256color"].includes(terminal) ||
      width <= 0 ||
      height <= 0
    )
      throw new TypeError("invalid terminal configuration");
    if (
      Buffer.byteLength(answerback) > 20 ||
      ![...answerback].every(
        (character) => character >= " " && character <= "~",
      )
    )
      throw new TypeError(
        "answerback must be at most 20 printable ASCII bytes",
      );
    const result = await this.driver.call(
      "session.open",
      { connectionId: this.id, terminal, width, height, answerback },
      { signal: options.signal },
    );
    return new Session(this.driver, result.sessionId);
  }
  close(options: OperationOptions = {}): Promise<void> {
    return (this.closePromise ??= this.driver
      .call("connection.close", { connectionId: this.id }, options)
      .then(() => {}));
  }
}

export class Session {
  private closePromise?: Promise<void>;
  constructor(
    private driver: Driver,
    readonly id: number,
  ) {}
  type(text: string, o: OperationOptions = {}) {
    return this.driver
      .call("session.send", { sessionId: this.id, text }, o)
      .then(() => {});
  }
  send(data: Uint8Array, o: OperationOptions = {}) {
    return this.driver
      .call(
        "session.send",
        { sessionId: this.id, base64: Buffer.from(data).toString("base64") },
        o,
      )
      .then(() => {});
  }
  press(key: Key, modifiers: Modifier[] = [], o: OperationOptions = {}) {
    return this.driver
      .call("session.press", { sessionId: this.id, key, modifiers }, o)
      .then(() => {});
  }
  resize(width: number, height: number, o: OperationOptions = {}) {
    if (width <= 0 || height <= 0)
      throw new TypeError("dimensions must be positive");
    return this.driver
      .call("session.resize", { sessionId: this.id, width, height }, o)
      .then(() => {});
  }
  async screen(o: OperationOptions = {}): Promise<Screen> {
    return new Screen(
      await this.driver.call("session.screen", { sessionId: this.id }, o),
    );
  }
  async waitFor(matcher: Matcher, o: WaitOptions = {}): Promise<Screen> {
    const timeout = o.timeout ?? this.driver.defaultTimeout;
    return new Screen(
      await this.driver.call(
        "session.wait",
        {
          sessionId: this.id,
          matcher,
          timeoutMilliseconds: timeout,
          stableMilliseconds: o.stableFor ?? 0,
        },
        { signal: o.signal, timeout: timeout + 1000 },
      ),
    );
  }
  waitForText(text: string, o: WaitOptions = {}) {
    return this.waitFor({ contains: text }, o);
  }
  waitForTextGone(text: string, o: WaitOptions = {}) {
    return this.waitFor({ not: { contains: text } }, o);
  }
  async waitForIdle(o: IdleOptions = {}): Promise<Screen> {
    const timeout = o.timeout ?? this.driver.defaultTimeout;
    return new Screen(
      await this.driver.call(
        "session.waitForIdle",
        {
          sessionId: this.id,
          timeoutMilliseconds: timeout,
          quietMilliseconds: o.quietFor ?? 100,
        },
        { signal: o.signal, timeout: timeout + 1000 },
      ),
    );
  }
  async subscribe(o: OperationOptions = {}): Promise<ScreenSubscription> {
    const r = await this.driver.call(
      "session.subscribe",
      { sessionId: this.id },
      o,
    );
    const s = new ScreenSubscription(this.driver, r.subscriptionId);
    this.driver.register(s);
    return s;
  }
  async subscribeEvents(o: OperationOptions = {}): Promise<EventSubscription> {
    const r = await this.driver.call(
      "session.subscribeEvents",
      { sessionId: this.id },
      o,
    );
    const s = new EventSubscription(this.driver, r.subscriptionId);
    this.driver.register(s);
    return s;
  }
  close(o: OperationOptions = {}): Promise<void> {
    return (this.closePromise ??= this.driver
      .call("session.close", { sessionId: this.id }, o)
      .then(() => {}));
  }
}

function validateConnection(c: ConnectionConfig) {
  if (!c.address) throw new TypeError("address is required");
  if (c.timeout !== undefined && c.timeout <= 0)
    throw new TypeError("timeout must be positive");
  if (c.protocol === "ssh") {
    if (!c.username) throw new TypeError("username is required");
    if (!c.password && !c.privateKey)
      throw new TypeError("password or privateKey is required");
    if (
      Number(Boolean(c.knownHostsFile)) +
        Number(Boolean(c.hostKeyFingerprint)) +
        Number(c.insecureSkipHostKeyCheck) !==
      1
    )
      throw new TypeError(
        "exactly one host-key verification option is required",
      );
  }
}
