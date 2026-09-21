import assert from "node:assert/strict";
import {
  After,
  AfterStep,
  Given,
  setDefaultTimeout,
  setWorldConstructor,
  Status,
  Then,
  When,
  World,
  type IWorldOptions,
} from "@cucumber/cucumber";
import {
  Attributes,
  Driver,
  Keys,
  type Connection,
  type Screen,
  type Session,
} from "@castingcode/tuicast";

setDefaultTimeout(40_000);
const password = "casting";

class State extends World {
  driver?: Driver;
  connection?: Connection;
  session?: Session;
  lastRevision?: number;

  constructor(options: IWorldOptions) {
    super(options);
  }
}
setWorldConstructor(State);

Given("I am connected to the reference TUI", async function (this: State) {
  this.driver = await Driver.launch({
    path: process.env.TUICAST_DRIVER ?? "tuicast-driver",
  });
  this.connection = await this.driver.connect({
    protocol: "ssh",
    address: process.env.TUICAST_REFERENCE_ADDRESS ?? "127.0.0.1:2222",
    username: "operator",
    password,
    insecureSkipHostKeyCheck: true,
  });
  this.session = await this.connection.openSession({
    terminal: "xterm-256color",
  });
  await this.session.waitForText("LOGIN / AUTHENTICATION", { stableFor: 50 });
});

Given("I am logged in", async function (this: State) {
  await this.session!.type("operator");
  await this.session!.press(Keys.Tab);
  await this.session!.type(password);
  await this.session!.press(Keys.Enter);
  await this.session!.waitForText("TERMINAL TEST SYSTEM");
});

When(
  /^I enter username "([^"]*)"$/,
  async function (this: State, value: string) {
    await this.session!.type(value);
  },
);

When(
  /^I enter password "([^"]*)"$/,
  async function (this: State, value: string) {
    await this.session!.press(Keys.Tab);
    await this.session!.type(value);
  },
);

When(/^I press (.+)$/, async function (this: State, name: string) {
  if (name === "Control+C") {
    await this.session!.press("c", ["Control"]);
    return;
  }
  const key = (Keys as Record<string, string>)[name];
  assert(key, `unsupported named key ${name}`);
  await this.session!.press(key);
});

const menuIndexes: Record<string, number> = {
  "Colors and Attributes": 4,
  "Function Keys": 6,
  Unicode: 10,
};

When(
  /^I select the "([^"]*)" menu option$/,
  async function (this: State, name: string) {
    const index = menuIndexes[name];
    assert.notEqual(index, undefined, `unsupported menu option ${name}`);
    for (let current = 0; current < index; current++) {
      await this.session!.press(Keys.ArrowDown);
    }
    await this.session!.press(Keys.Enter);
  },
);

const headings: Record<string, string> = {
  login: "LOGIN / AUTHENTICATION",
  "main menu": "TERMINAL TEST SYSTEM",
  "Colors and Attributes": "ANSI COLORS AND ATTRIBUTES",
  "Function Keys": "FUNCTION KEYS",
  Unicode: "UNICODE ALIGNMENT",
};

Then(
  /^the application is on the "([^"]*)" screen$/,
  async function (this: State, name: string) {
    const heading = headings[name];
    assert(heading, `unsupported screen ${name}`);
    await this.session!.waitForText(heading, { stableFor: 50 });
  },
);

Then(
  /^the screen contains "([^"]*)"$/,
  async function (this: State, text: string) {
    await this.session!.waitForText(text);
  },
);

Then(
  "the application reports that authentication failed",
  async function (this: State) {
    await this.session!.waitForText("Invalid user ID or password");
  },
);

Then(
  /^"([^"]*)" begins at zero-based column (\d+) and row (\d+)$/,
  async function (this: State, text: string, column: string, row: string) {
    assert.deepEqual((await this.session!.screen()).find(text), {
      column: Number(column),
      row: Number(row),
    });
  },
);

Then(
  /^"([^"]*)" is rendered with the "([^"]*)" attribute$/,
  async function (this: State, text: string, attribute: string) {
    const screen = await this.session!.screen();
    const position = screen.find(text);
    assert(position, `${text} was not found`);
    const expected =
      attribute === "bold" ? Attributes.Bold : Attributes.Underline;
    assert(screen.cellAt(position.column, position.row)!.attributes & expected);
  },
);

Then(
  /^the "([^"]*)" color sample uses foreground (\d+) and background (\d+)$/,
  async function (
    this: State,
    text: string,
    foreground: string,
    background: string,
  ) {
    const screen = await this.session!.screen();
    const position = screen.find(text);
    assert(position, `${text} was not found`);
    const cell = screen.cellAt(position.column, position.row)!;
    assert.equal(cell.foreground, Number(foreground));
    assert.equal(cell.background, Number(background));
  },
);

Then(
  /^the "([^"]*)" sample renders "([^"]*)" at zero-based column (\d+) and row (\d+)$/,
  async function (
    this: State,
    label: string,
    text: string,
    column: string,
    row: string,
  ) {
    const screen = await this.session!.screen();
    const expectedRow = Number(row);
    assert.equal(screen.find(label)?.row, expectedRow);
    assert.deepEqual(screen.find(text), {
      column: Number(column),
      row: expectedRow,
    });
  },
);

Then(
  /^the captured key count is (\d+)$/,
  async function (this: State, count: string) {
    await this.session!.waitForText(`Captured count: ${count}`);
  },
);

Then(
  /^the latest key is "([^"]*)"$/,
  async function (this: State, key: string) {
    await this.session!.waitForText(`Last: ${key}`);
  },
);

Then(
  /^the latest key modifiers are "([^"]*)"$/,
  async function (this: State, modifiers: string) {
    await this.session!.waitForText(`Modifiers: ${modifiers}`);
  },
);

AfterStep(async function (this: State, { result, pickleStep }) {
  if (!this.session || result?.status === Status.SKIPPED) return;
  const failed = result?.status !== Status.PASSED;
  const isOutcome = pickleStep.type === "Outcome";
  if (!failed && !isOutcome) return;

  const screen = await this.session.screen();
  if (!failed && screen.revision === this.lastRevision) return;
  const prefix = failed ? "failed-terminal" : "terminal";
  this.attach(terminalCaptureHTML(screen), {
    mediaType: "text/html",
    fileName: `${prefix}-revision-${screen.revision}.html`,
  });
  if (failed) {
    this.attach(screen.text, {
      mediaType: "text/plain",
      fileName: `failed-terminal-revision-${screen.revision}.txt`,
    });
  }
  this.lastRevision = screen.revision;
});

After(async function (this: State) {
  try {
    if (this.session) {
      const screen = await this.session.screen();
      if (!screen.contains("LOGIN / AUTHENTICATION")) {
        if (!screen.contains("TERMINAL TEST SYSTEM")) {
          await this.session.press(Keys.Escape);
          await this.session.waitForText("TERMINAL TEST SYSTEM");
        }
        await this.session.press(Keys.F2);
        await this.session.waitForText("Signed out");
      }
      await this.session.close();
    }
  } finally {
    await this.connection?.close().catch(() => {});
    await this.driver?.close().catch(() => {});
  }
});

const cellWidth = 9;
const cellHeight = 18;
const padding = 12;
const defaultForeground = "#d8dee9";
const defaultBackground = "#1e1e2e";
const ansiColors = [
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
];

function terminalCaptureHTML(screen: Screen): string {
  const width = screen.width * cellWidth + padding * 2;
  const height = screen.height * cellHeight + padding * 2;
  let capture = `<div class="tuicast-terminal-capture"><div style="margin:0 0 6px;color:#555;font:12px sans-serif">Terminal ${screen.width}x${screen.height} · revision ${screen.revision}</div>`;
  capture += `<svg xmlns="http://www.w3.org/2000/svg" role="img" aria-label="Terminal screen at revision ${screen.revision}" viewBox="0 0 ${width} ${height}" width="${width}" height="${height}" style="max-width:100%;height:auto;background:${defaultBackground};border-radius:6px">`;

  for (let row = 0; row < screen.height; row++) {
    for (let column = 0; column < screen.width; column++) {
      const cell = screen.cellAt(column, row);
      if (!cell || cell.width === 0) continue;
      let foreground = captureColor(cell.foreground, defaultForeground);
      let background = captureColor(cell.background, defaultBackground);
      if (cell.attributes & Attributes.Reverse) {
        [foreground, background] = [background, foreground];
      }
      const x = padding + column * cellWidth;
      const y = padding + row * cellHeight;
      const width = Math.max(1, cell.width) * cellWidth;
      if (background !== defaultBackground) {
        capture += `<rect x="${x}" y="${y}" width="${width}" height="${cellHeight}" fill="${background}"/>`;
      }
      if (
        cell.text === "" ||
        cell.text === " " ||
        cell.attributes & Attributes.Conceal
      ) {
        continue;
      }
      const weight = cell.attributes & Attributes.Bold ? "bold" : "normal";
      const decoration =
        cell.attributes & Attributes.Underline ? "underline" : "none";
      capture += `<text x="${x}" y="${y + 14}" fill="${foreground}" font-family="ui-monospace,SFMono-Regular,Menlo,Consolas,monospace" font-size="14" font-weight="${weight}" text-decoration="${decoration}">${escapeHTML(cell.text)}</text>`;
    }
  }
  if (screen.cursor.visible) {
    const x = padding + screen.cursor.column * cellWidth;
    const y = padding + screen.cursor.row * cellHeight;
    capture += `<rect x="${x}" y="${y}" width="${cellWidth}" height="${cellHeight}" fill="none" stroke="#ffffff" stroke-opacity="0.75"/>`;
  }
  return `${capture}</svg></div>`;
}

function captureColor(index: number, fallback: string): string {
  if (index < 0) return fallback;
  if (index < ansiColors.length) return ansiColors[index]!;
  if (index <= 231) {
    const value = index - 16;
    const levels = [0, 95, 135, 175, 215, 255];
    return rgb(
      levels[Math.floor(value / 36)]!,
      levels[Math.floor(value / 6) % 6]!,
      levels[value % 6]!,
    );
  }
  if (index <= 255) {
    const level = 8 + (index - 232) * 10;
    return rgb(level, level, level);
  }
  return fallback;
}

function rgb(red: number, green: number, blue: number): string {
  return `#${[red, green, blue].map((value) => value.toString(16).padStart(2, "0")).join("")}`;
}

function escapeHTML(value: string): string {
  return value.replace(
    /[&<>]/g,
    (character) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;" })[character]!,
  );
}
