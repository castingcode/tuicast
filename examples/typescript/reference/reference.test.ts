import assert from "node:assert/strict";
import test from "node:test";
import {
  all,
  Attributes,
  contains,
  Driver,
  isTimeout,
  Keys,
  not,
  WaitError,
  type Connection,
  type ConnectionConfig,
  type Session,
} from "@castingcode/tuicast";

const password = "casting";
const driverPath = process.env.TUICAST_DRIVER ?? "tuicast-driver";
const sshAddress = process.env.TUICAST_REFERENCE_ADDRESS ?? "127.0.0.1:2222";
const telnetAddress =
  process.env.TUICAST_REFERENCE_TELNET_ADDRESS ?? "127.0.0.1:2323";

interface Fixture {
  driver: Driver;
  connection: Connection;
  close(): Promise<void>;
}

async function launch(config?: ConnectionConfig): Promise<Fixture> {
  const driver = await Driver.launch({ path: driverPath });
  try {
    const connection = await driver.connect(
      config ?? {
        protocol: "ssh",
        address: sshAddress,
        username: "operator",
        password,
        insecureSkipHostKeyCheck: true,
      },
    );
    return {
      driver,
      connection,
      async close() {
        await connection.close();
        await driver.close();
      },
    };
  } catch (error) {
    await driver.close();
    throw error;
  }
}

async function login(session: Session): Promise<void> {
  await session.waitForText("LOGIN / AUTHENTICATION", { stableFor: 50 });
  await session.type("operator");
  await session.press(Keys.Tab);
  await session.type(password);
  await session.press(Keys.Enter);
  await session.waitForText("TERMINAL TEST SYSTEM");
}

async function openScenario(session: Session, index: number, heading: string) {
  for (let current = 0; current < index; current++) {
    await session.press(Keys.ArrowDown);
  }
  await session.press(Keys.Enter);
  return session.waitForText(heading, { stableFor: 50 });
}

async function withSession(
  run: (session: Session, fixture: Fixture) => Promise<void>,
  options: Parameters<Connection["openSession"]>[0] = {},
): Promise<void> {
  const fixture = await launch();
  let session: Session | undefined;
  try {
    session = await fixture.connection.openSession(options);
    await run(session, fixture);
  } finally {
    await session?.close();
    await fixture.close();
  }
}

test("a receiving form can be validated, completed, and submitted", async () => {
  await withSession(
    async (session) => {
      await login(session);
      const initial = await openScenario(session, 1, "RECEIVING FORM");
      assert(initial.contains("Purchase order"));

      await session.press(Keys.Tab, ["Shift"]);
      await session.press(Keys.Tab, ["Shift"]);
      await session.press(Keys.Enter);
      await session.waitFor(
        all(
          contains("purchase order is required"),
          contains("quantity must be a positive number"),
        ),
      );

      for (const value of [
        "PO-10002341",
        "WIDGET-42",
        "25",
        "A-01-02",
        "dock 3",
      ]) {
        await session.type(value);
        await session.press(Keys.Tab);
      }
      await session.press(Keys.ArrowRight);
      await session.press(Keys.Tab);
      await session.press(Keys.Enter);

      const screen = await session.waitForText("RECEIPT ACCEPTED");
      assert(
        screen.contains(
          "PO-10002341 / WIDGET-42 / quantity 25 / A-01-02 / Urgent",
        ),
      );
    },
    { terminal: "xterm-256color" },
  );
});

test("warehouse orders can be filtered, navigated, and inspected", async () => {
  await withSession(async (session) => {
    await login(session);
    await openScenario(session, 2, "WAREHOUSE ORDERS");
    await session.press("/");
    await session.type("HOLD");
    await session.press(Keys.Enter);
    await session.press(Keys.End);
    await session.press(Keys.Enter);

    const screen = await session.waitFor(
      all(contains("ORDER DETAILS"), contains("Status:    HOLD")),
    );
    assert(screen.contains("Location:"));
  });
});

test("independent sessions share one authenticated SSH connection", async () => {
  const fixture = await launch();
  try {
    await Promise.all(
      [
        [1, "RECEIVING FORM"],
        [2, "WAREHOUSE ORDERS"],
        [6, "FUNCTION KEYS"],
      ].map(async ([index, heading]) => {
        const session = await fixture.connection.openSession();
        try {
          await login(session);
          const screen = await openScenario(
            session,
            index as number,
            heading as string,
          );
          assert(screen.contains(heading as string));
        } finally {
          await session.close();
        }
      }),
    );
  } finally {
    await fixture.close();
  }
});

test("a screen subscription observes revisions without polling", async () => {
  await withSession(async (session) => {
    await login(session);
    const subscription = await session.subscribe();
    try {
      await openScenario(session, 1, "RECEIVING FORM");
      let observed;
      for await (const update of subscription) {
        if (update.contains("RECEIVING FORM")) {
          observed = update;
          break;
        }
      }
      assert(observed);
      assert(observed.contains("RECEIVING FORM"));
      assert(observed.revision > 0);
    } finally {
      await subscription.close();
    }
  });
});

test("BELL and ENQ events arrive in order after configured answerback", async () => {
  await withSession(
    async (session) => {
      await login(session);
      await openScenario(session, 11, "BELL / ENQ ANSWERBACK");
      const subscription = await session.subscribeEvents();
      const events = subscription[Symbol.asyncIterator]();
      try {
        await session.press("b");
        assert.deepEqual((await events.next()).value, {
          sequence: 1,
          type: "bell",
        });
        await session.press("e");
        assert.deepEqual((await events.next()).value, {
          sequence: 2,
          type: "enquiry",
          data: "TUICAST-ANSWER",
        });

        const screen = await session.waitForText(
          'Answerback: "TUICAST-ANSWER"',
        );
        assert(screen.contains("BEL emitted: 1 / 1"));
        assert(screen.contains("ENQ emitted: 1 / 1"));
      } finally {
        await subscription.close();
      }
    },
    { answerback: "TUICAST-ANSWER" },
  );
});

test("automation waits for a complete stable partial screen", async () => {
  await withSession(async (session) => {
    await login(session);
    await openScenario(session, 7, "PARTIAL SCREEN UPDATES");
    await session.press("1");
    await session.waitForText("READY");
    const screen = await session.waitFor(
      all(contains("READY"), contains("SCREEN COMPLETE / INPUT ENABLED")),
      { stableFor: 100 },
    );
    assert(screen.contains("Inventory: VERIFIED"));
    await session.waitForIdle({ quietFor: 100 });
    await session.press("x");
    await session.waitForText("Accepted input: x");
  });
});

test("a finite long-running operation reaches semantic completion", async () => {
  await withSession(async (session) => {
    await login(session);
    await openScenario(session, 8, "LONG-RUNNING OPERATION");
    await session.press("1");
    const screen = await session.waitFor(
      all(
        contains("Processed: 120"),
        contains("Status: OPERATION COMPLETE"),
        not(contains("STREAMING")),
      ),
      { timeout: 12_000 },
    );
    assert(screen.contains("Remaining: 0"));
  });
});

test("a timed-out wait retains its condition and last screen", async () => {
  await withSession(async (session) => {
    await assert.rejects(
      session.waitForText("TEXT THAT WILL NOT APPEAR", { timeout: 100 }),
      (error: unknown) => {
        assert(isTimeout(error));
        assert(error instanceof WaitError);
        assert.match(error.expected, /TEXT THAT WILL NOT APPEAR/);
        assert(error.lastScreen.contains("LOGIN / AUTHENTICATION"));
        return true;
      },
    );
  });
});

test("the same workflow runs over Telnet", async () => {
  const fixture = await launch({ protocol: "telnet", address: telnetAddress });
  const session = await fixture.connection.openSession();
  try {
    await login(session);
    const screen = await openScenario(session, 1, "RECEIVING FORM");
    assert(screen.contains("Purchase order"));
  } finally {
    await session.close();
    await fixture.close();
  }
});

test("resize changes layout while preserving application state", async () => {
  await withSession(
    async (session) => {
      await login(session);
      await openScenario(session, 9, "TERMINAL RESIZE");
      await session.type("value survives");
      await session.press(Keys.ArrowDown);
      await session.press("T");
      await session.resize(132, 24);
      const screen = await session.waitFor(
        all(
          contains("Actual dimensions: 132x24"),
          contains("Status: MATCH"),
          contains("value survives"),
          contains("> ORD-10002342"),
        ),
      );
      assert.equal(screen.width, 132);
    },
    { width: 80, height: 24 },
  );
});

test("colors, attributes, and wide Unicode are structured cells", async () => {
  const fixture = await launch();
  const colors = await fixture.connection.openSession({
    terminal: "xterm-256color",
  });
  let unicode: Session | undefined;
  try {
    await login(colors);
    let screen = await openScenario(colors, 4, "ANSI COLORS AND ATTRIBUTES");
    const bold = screen.find("BOLD");
    assert(bold);
    assert(screen.cellAt(bold.column, bold.row)!.attributes & Attributes.Bold);

    unicode = await fixture.connection.openSession({
      terminal: "xterm-256color",
    });
    await login(unicode);
    screen = await openScenario(unicode, 10, "UNICODE ALIGNMENT");
    const position = screen.find("漢字");
    assert(position);
    assert.equal(screen.cellAt(position.column, position.row)?.width, 2);
    assert.equal(screen.cellAt(position.column + 1, position.row)?.width, 0);
  } finally {
    await unicode?.close();
    await colors.close();
    await fixture.close();
  }
});

test("named keys and modifiers are encoded for the terminal", async () => {
  await withSession(
    async (session) => {
      await login(session);
      await openScenario(session, 6, "FUNCTION KEYS");
      await session.press(Keys.F5);
      await session.waitForText("Last: f5");
      await session.press("c", ["Control"]);
      const screen = await session.waitForText("Last: ctrl+c");
      assert(screen.contains("Modifiers: control"));
    },
    { terminal: "xterm-256color" },
  );
});
