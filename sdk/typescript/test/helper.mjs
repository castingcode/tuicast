import { createInterface } from "node:readline";
const cell = (text) => ({
  text,
  width: 1,
  foreground: -1,
  background: -1,
  attributes: 0,
});
const screen = (revision = 3) => ({
  width: 5,
  height: 1,
  revision,
  text: "READY",
  cursor: { column: 0, row: 0, visible: true },
  cells: [..."READY"].map(cell),
});
const rl = createInterface({ input: process.stdin });
const send = (x) => process.stdout.write(JSON.stringify(x) + "\n");
rl.on("line", (line) => {
  const q = JSON.parse(line),
    r = { jsonrpc: "2.0", id: q.id };
  if (process.env.BAD === "json") {
    process.stdout.write("nope\n");
    return;
  }
  if (process.env.BAD === "version") {
    send({ ...r, jsonrpc: "1.0", result: {} });
    return;
  }
  switch (q.method) {
    case "driver.ping":
      r.result = { protocolVersion: process.env.BAD ?? "1" };
      break;
    case "driver.shutdown":
      r.result = { shuttingDown: true };
      send(r);
      process.exit();
    case "connection.open":
      r.result = { connectionId: 11 };
      break;
    case "connection.close":
    case "session.close":
      r.result = { closed: true };
      break;
    case "session.open":
      r.result = { sessionId: 22 };
      break;
    case "session.screen":
      r.result = screen();
      setTimeout(() => send(r), q.id % 2 ? 5 : 0);
      return;
    case "session.send":
      r.result = { bytesSent: 1 };
      break;
    case "session.press":
      r.result = { sent: true };
      break;
    case "session.resize":
      r.result = { resized: true };
      break;
    case "session.waitForIdle":
      r.result = screen();
      break;
    case "session.wait":
      if (q.params.matcher.contains === "MISSING") {
        r.error = {
          code: -32000,
          message: "timed out",
          data: { kind: "timeout", expected: "missing", screen: screen() },
        };
      } else r.result = screen();
      break;
    case "session.subscribe":
      r.result = { subscriptionId: 33 };
      send(r);
      send({
        jsonrpc: "2.0",
        method: "session.screen",
        params: { subscriptionId: 33, screen: screen(1) },
      });
      send({
        jsonrpc: "2.0",
        method: "session.screen",
        params: { subscriptionId: 33, screen: screen(2) },
      });
      return;
    case "session.subscribeEvents":
      r.result = { subscriptionId: 34 };
      send(r);
      send({
        jsonrpc: "2.0",
        method: "session.event",
        params: { subscriptionId: 34, event: { sequence: 1, type: "bell" } },
      });
      return;
    case "session.unsubscribe":
      r.result = { unsubscribed: true };
      break;
    default:
      r.error = { code: -32601, message: "unknown" };
  }
  send(r);
});
