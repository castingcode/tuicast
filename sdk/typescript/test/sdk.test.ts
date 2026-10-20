import test from "node:test";
import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import {
  Driver,
  Screen,
  WaitError,
  isTimeout,
  Attributes,
  Color,
  hasAttributes,
  all,
  contains,
  not,
  Keys,
} from "../src/index.js";
const helper = fileURLToPath(new URL("helper.mjs", import.meta.url));
test("shared detached screen queries", async () => {
  const fixture = JSON.parse(
    await readFile(
      new URL(
        "../../../../schema/testdata/screen-queries.json",
        import.meta.url,
      ),
      "utf8",
    ),
  );
  for (const c of fixture.cases) {
    const s = new Screen(c.screen);
    assert(s.contains(c.contains));
    assert.deepEqual(s.find(c.find), c.position);
    assert.equal(s.find("MISSING"), undefined);
    assert(s.line(c.position.row));
    assert.equal(s.cellAt(-1, 0), undefined);
    assert(Object.isFrozen(s));
  }
});
test("lifecycle, concurrent correlation, waits and subscriptions", async () => {
  const d = await Driver.launch({
    path: process.execPath,
    args: [helper],
    defaultTimeout: 1000,
  });
  const c = await d.connect({ protocol: "telnet", address: "local" });
  const s = await c.openSession();
  assert.equal(s.id, 22);
  await Promise.all(
    Array.from({ length: 12 }, () =>
      s.screen().then((x) => {
        assert.equal(x.text, "READY");
        assert.equal(x.title, "RF-01");
      }),
    ),
  );
  await s.type("x");
  await s.send(new Uint8Array([1]));
  await s.press(Keys.Enter);
  await s.resize(10, 2);
  assert(
    (await s.waitFor(all(contains("READY"), not(contains("NO"))))).contains(
      "READY",
    ),
  );
  assert.equal((await s.waitForIdle()).revision, 3);
  const screens = await s.subscribe();
  assert((await screens[Symbol.asyncIterator]().next()).value!.revision > 0);
  await screens.close();
  await screens.close();
  const events = await s.subscribeEvents();
  const eventIterator = events[Symbol.asyncIterator]();
  assert.equal((await eventIterator.next()).value?.type, "bell");
  assert.deepEqual((await eventIterator.next()).value, {
    sequence: 2,
    type: "unsupported",
    data: "CSI ? 2004 h",
  });
  await events.close();
  await assert.rejects(
    s.waitForText("MISSING"),
    (e) =>
      e instanceof WaitError &&
      isTimeout(e) &&
      e.lastScreen.text === "READY" &&
      e.dump === '{"type":"header"}\n' &&
      e.dumpPath === "/tmp/tuicast-session-1-1.jsonl",
  );
  assert.deepEqual(await s.state(), { ended: false });
  assert.deepEqual(await s.waitForEnd({ timeout: 250 }), {
    ended: true,
    error: "reading terminal output: reset",
  });
  await s.close();
  await s.close();
  await c.close();
  await d.close();
  await d.close();
});
test("configuration, cancellation and protocol failures", async () => {
  const d = await Driver.launch({
    path: process.execPath,
    args: [helper],
    defaultTimeout: 100,
  });
  await assert.rejects(
    d.connect({ protocol: "ssh", address: "x", username: "u", password: "p" }),
    /host-key/,
  );
  await assert.rejects(
    d.connect({ protocol: "telnet", address: "", timeout: 1 }),
    /address/,
  );
  const c = await d.connect({
    protocol: "ssh",
    address: "x",
    username: "u",
    password: "p",
    insecureSkipHostKeyCheck: true,
  });
  await assert.rejects(c.openSession({ answerback: "\n" }), /answerback/);
  const s = await c.openSession();
  await assert.rejects(s.screen({ signal: AbortSignal.abort() }));
  assert.equal(Attributes.Bold, 1);
  const cell = {
    text: "A",
    width: 1,
    foreground: Color.Red,
    background: Color.Default,
    attributes: Attributes.Bold | Attributes.Underline,
  };
  assert.equal(cell.foreground, 1);
  assert.equal(cell.background, -1);
  assert(hasAttributes(cell, Attributes.Bold | Attributes.Underline));
  assert(!hasAttributes(cell, Attributes.Bold | Attributes.Reverse));
  await d.close();
  for (const bad of ["2", "json", "version"]) {
    await assert.rejects(
      Driver.launch({
        path: process.execPath,
        args: [helper],
        env: { BAD: bad },
        defaultTimeout: 100,
      }),
      /protocol/,
    );
  }
});
