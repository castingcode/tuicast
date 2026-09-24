const { spawnSync } = require("node:child_process");
const path = require("node:path");

module.exports = function run(binary) {
  const executable = path.join(
    __dirname,
    "..",
    "vendor",
    process.platform === "win32" ? `${binary}.exe` : binary,
  );
  const result = spawnSync(executable, process.argv.slice(2), {
    stdio: "inherit",
    windowsHide: false,
  });

  if (result.error) {
    console.error(`Unable to start ${binary}: ${result.error.message}`);
    process.exit(1);
  }

  if (result.signal) {
    process.kill(process.pid, result.signal);
  }

  process.exit(result.status ?? 1);
};
