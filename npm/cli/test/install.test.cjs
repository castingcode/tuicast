const { describe, it } = require("node:test");
const assert = require("node:assert/strict");

const { artifactName, checksumFor, releaseTarget } = require("../scripts/install.cjs");

describe("TUICast CLI installer", () => {
  it("maps Node platform names to asymmetric release targets", () => {
    assert.deepEqual(releaseTarget("win32", "x64"), {
      os: "windows",
      arch: "amd64",
    });
    assert.deepEqual(releaseTarget("darwin", "arm64"), {
      os: "darwin",
      arch: "arm64",
    });
  });

  it("rejects platforms without release artifacts", () => {
    assert.throws(() => releaseTarget("freebsd", "x64"), /Unsupported platform/);
    assert.throws(() => releaseTarget("linux", "ia32"), /Unsupported platform/);
  });

  it("uses GoReleaser artifact names including the Windows extension", () => {
    assert.equal(
      artifactName("reference-tui", "1.2.3", "win32", "arm64"),
      "reference-tui_1.2.3_windows_arm64.exe",
    );
    assert.equal(
      artifactName("tuicast-driver", "1.2.3", "linux", "x64"),
      "tuicast-driver_1.2.3_linux_amd64",
    );
  });

  it("matches an exact artifact instead of a similarly named one", () => {
    const wanted = "a".repeat(64);
    const other = "b".repeat(64);
    const checksums = `${other}  tuicast-driver_1.2.3_linux_arm64\n${wanted}  tuicast-driver_1.2.3_linux_amd64\n`;
    assert.equal(checksumFor(checksums, "tuicast-driver_1.2.3_linux_amd64"), wanted);
  });
});
