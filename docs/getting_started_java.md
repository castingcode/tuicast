# Getting started with TUICast for Java

TUICast lets Java applications control interactive terminal applications over
SSH or Telnet. The Java SDK launches `tuicast-driver`, which owns the network
connection and terminal emulation.

## Prerequisites

- Java 21 or newer
- Apache Maven
- `tuicast-driver` installed and available on `PATH`

On macOS or Linux, install the driver and the reference TUI used by this guide:

```sh
curl --proto '=https' --tlsv1.2 -LsSf \
  https://github.com/castingcode/tuicast/releases/latest/download/install.sh |
  sh -s -- --component all --bin-dir "$HOME/.local/bin"
export PATH="$HOME/.local/bin:$PATH"
```

See the [main installation instructions](../README.md#driver-and-other-binaries---using-the-install-script)
for Windows and other installation options.

## Install the SDK

Create a Maven project:

```sh
mkdir -p tuicast-java-demo/src/main/java
cd tuicast-java-demo
```

Create `pom.xml` with the TUICast dependency from Maven Central:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>
  <groupId>example</groupId>
  <artifactId>tuicast-java-demo</artifactId>
  <version>1.0.0</version>
  <properties>
    <maven.compiler.release>21</maven.compiler.release>
    <project.build.sourceEncoding>UTF-8</project.build.sourceEncoding>
  </properties>
  <dependencies>
    <dependency>
      <groupId>dev.castingcode</groupId>
      <artifactId>tuicast</artifactId>
      <version>0.0.1</version>
    </dependency>
  </dependencies>
  <build>
    <plugins>
      <plugin>
        <groupId>org.codehaus.mojo</groupId>
        <artifactId>exec-maven-plugin</artifactId>
        <version>3.5.0</version>
      </plugin>
    </plugins>
  </build>
</project>
```

## Start the reference TUI

In a separate terminal, start TUICast's deterministic demonstration server:

```sh
reference-tui \
  --ssh-address 127.0.0.1:2222 \
  --ssh-username demo \
  --ssh-password demo-password
```

The SSH transport uses `demo` / `demo-password`. The application displayed
inside the terminal has its own test login: `operator` / `casting`.

## Automate a login

Create `src/main/java/Quickstart.java`:

```java
import dev.castingcode.tuicast.Driver;
import dev.castingcode.tuicast.Key;
import dev.castingcode.tuicast.SshConfig;
import dev.castingcode.tuicast.TerminalProfile;
import dev.castingcode.tuicast.SessionConfig;

public final class Quickstart {
  public static void main(String[] args) {
    var ssh =
        SshConfig.builder("127.0.0.1:2222", "demo")
            .password("demo-password")
            // Safe only because this example connects to a local test server.
            .insecureSkipHostKeyCheck(true)
            .build();

    try (var driver = Driver.launch();
        var connection = driver.connect(ssh);
        var session =
            connection.openSession(new SessionConfig(TerminalProfile.XTERM_256_COLOR, 80, 24, ""))) {
      session.waitForText("LOGIN / AUTHENTICATION");
      session.type("operator");
      session.press(Key.Tab);
      session.type("casting");
      session.press(Key.Enter);

      var screen = session.waitForText("TERMINAL TEST SYSTEM");
      if (!screen.contains("Authenticated as operator")) {
        throw new IllegalStateException("login did not complete");
      }

      System.out.println("Login succeeded");
    }
  }
}
```

Compile and run the example:

```sh
mvn compile exec:java -Dexec.mainClass=Quickstart
```

`waitForText` waits against the emulated terminal screen rather than raw
network output. The returned `Screen` is an immutable snapshot that can also
inspect lines, cells, colors, attributes, and the cursor. Try-with-resources
closes the session, connection, and driver even when an operation fails.

For a real SSH server, replace `insecureSkipHostKeyCheck(true)` with exactly one
trusted host-key option: `knownHostsFile(...)` or `hostKeyFingerprint(...)`.
Keep application and SSH credentials in environment variables or a secret
manager.
