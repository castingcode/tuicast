const crypto = require("node:crypto");
const fs = require("node:fs");
const https = require("node:https");
const path = require("node:path");

const repository = "castingcode/tuicast";
const binaries = ["tuicast-driver", "reference-tui"];

function releaseTarget(platform = process.platform, architecture = process.arch) {
  const operatingSystems = {
    darwin: "darwin",
    linux: "linux",
    win32: "windows",
  };
  const architectures = {
    arm64: "arm64",
    x64: "amd64",
  };
  const os = operatingSystems[platform];
  const arch = architectures[architecture];
  if (!os || !arch) {
    throw new Error(`Unsupported platform: ${platform}/${architecture}`);
  }
  return { os, arch };
}

function artifactName(binary, version, platform = process.platform, architecture = process.arch) {
  const { os, arch } = releaseTarget(platform, architecture);
  const extension = os === "windows" ? ".exe" : "";
  return `${binary}_${version}_${os}_${arch}${extension}`;
}

function checksumFor(contents, artifact) {
  for (const line of contents.split(/\r?\n/)) {
    const match = line.match(/^([0-9a-fA-F]{64})\s+\*?(.+)$/);
    if (match && match[2] === artifact) {
      return match[1].toLowerCase();
    }
  }
  throw new Error(`Checksum not found for ${artifact}`);
}

function download(url, destination, redirectsRemaining = 5) {
  return new Promise((resolve, reject) => {
    const request = https.get(
      url,
      { headers: { "User-Agent": "@castingcode/tuicast-cli" } },
      (response) => {
        if (
          response.statusCode >= 300 &&
          response.statusCode < 400 &&
          response.headers.location
        ) {
          response.resume();
          if (redirectsRemaining === 0) {
            reject(new Error(`Too many redirects downloading ${url}`));
            return;
          }
          download(
            new URL(response.headers.location, url).toString(),
            destination,
            redirectsRemaining - 1,
          ).then(resolve, reject);
          return;
        }
        if (response.statusCode !== 200) {
          response.resume();
          reject(new Error(`Download failed with HTTP ${response.statusCode}: ${url}`));
          return;
        }

        const output = fs.createWriteStream(destination, { mode: 0o755 });
        response.pipe(output);
        response.on("error", reject);
        output.on("finish", () => output.close(resolve));
        output.on("error", reject);
      },
    );
    request.setTimeout(30_000, () => request.destroy(new Error(`Download timed out: ${url}`)));
    request.on("error", reject);
  });
}

async function install() {
  const packageRoot = path.join(__dirname, "..");
  const { version } = require(path.join(packageRoot, "package.json"));
  if (version === "0.0.0-development") {
    throw new Error("The development package cannot install a release binary");
  }

  const releaseURL = `https://github.com/${repository}/releases/download/v${version}`;
  const vendorDirectory = path.join(packageRoot, "vendor");
  const checksums = path.join(vendorDirectory, "checksums.txt");

  fs.mkdirSync(vendorDirectory, { recursive: true });
  try {
    await download(`${releaseURL}/checksums.txt`, checksums);
    const checksumContents = fs.readFileSync(checksums, "utf8");
    for (const binary of binaries) {
      const artifact = artifactName(binary, version);
      const destination = path.join(
        vendorDirectory,
        process.platform === "win32" ? `${binary}.exe` : binary,
      );
      const temporary = `${destination}.download`;
      try {
        await download(`${releaseURL}/${artifact}`, temporary);
        const expected = checksumFor(checksumContents, artifact);
        const actual = crypto.createHash("sha256").update(fs.readFileSync(temporary)).digest("hex");
        if (actual !== expected) {
          throw new Error(`Checksum verification failed for ${artifact}`);
        }
        fs.rmSync(destination, { force: true });
        fs.renameSync(temporary, destination);
        fs.chmodSync(destination, 0o755);
      } finally {
        fs.rmSync(temporary, { force: true });
      }
    }
    console.log(`Installed TUICast CLI ${version} for ${process.platform}/${process.arch}`);
  } finally {
    fs.rmSync(checksums, { force: true });
  }
}

if (require.main === module) {
  install().catch((error) => {
    console.error(`Unable to install TUICast CLI: ${error.message}`);
    process.exitCode = 1;
  });
}

module.exports = { artifactName, checksumFor, releaseTarget };
