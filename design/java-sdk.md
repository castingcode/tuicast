# Java SDK

The Java 21 SDK in `sdk/java` is a thin, thread-safe JSON-RPC v1 client for a
child `tuicast-driver`. It uses one reader virtual thread, correlates concurrent
out-of-order replies, and exposes `Driver`, `Connection`, `Session`, immutable
`Screen` snapshots, typed SSH/Telnet configuration, matchers, waits, and bounded
screen/event subscriptions. All lifecycle types are idempotent `AutoCloseable`s.

Install Java and Maven with SDKMAN when needed:

```sh
sdk install java 21.0.6-tem
sdk use java 21.0.6-tem
sdk install maven 3.9.9
```

The baseline is exactly Java 21 (`--release 21`). Jackson is the sole runtime
dependency. Coordinates are `dev.castingcode:tuicast:0.0.1`; metadata
is present for Maven Central publication.

```java
try (var driver = Driver.launch();
     var connection = driver.connect(new TelnetConfig("host:23"));
     var session = connection.openSession()) {
  session.waitForText("User ID:");
  session.type("operator");
  session.press(Key.Enter);
}
```

## Development and verification

```sh
cd sdk/java
mvn install              # SDK tests, coverage/format checks, and local install
cd ../../examples/java
mvn verify               # JUnit reference integration and formatting checks
mvn test -Pcucumber      # shared Cucumber features only; writes target/cucumber.json
# target/cucumber.json is accepted by examples/reporting's existing generator
```

These three commands are the exact CI sequence and keep SDK, reference
integration, and Cucumber failures distinct. The SDK enforces 80% line and 60%
branch coverage. Run `mvn spotless:apply` in either Maven directory to apply
formatting.

Set `TUICAST_DRIVER`, `TUICAST_REFERENCE_ADDRESS`, and (for Telnet-oriented
tests) `TUICAST_REFERENCE_TELNET_ADDRESS` as for the Go examples. Java Cucumber
loads `examples/features` as a test resource rather than copying it. Its JSON
plugin writes `target/cucumber.json`; failures attach plain text and an inline
SVG before cleanup, while successful input steps are never captured.

## Publishing

Java releases use the Central Publisher Portal through
`.github/workflows/publish-java.yml`. Unlike npm and PyPI, Central does not
support GitHub OIDC trusted publishing, so the protected `maven-central`
GitHub environment must contain these secrets:

- `MAVEN_CENTRAL_USERNAME` and `MAVEN_CENTRAL_PASSWORD`: the username and
  password generated together as a Central Portal user token.
- `MAVEN_GPG_PRIVATE_KEY`: the ASCII-armored private signing key.
- `MAVEN_GPG_PASSPHRASE`: the private key's passphrase.

Register and verify the `dev.castingcode` namespace in the Central Portal by
adding its supplied TXT record to `castingcode.dev`. Publish the signing key's
public half to a Central-supported keyserver before the first release. Keep the
private key and its revocation certificate outside the repository.

The package version in `pom.xml` must match a tag named
`sdk/java/v<version>`. The workflow runs the complete CI suite, builds source
and Javadoc JARs, signs every artifact, and uploads the bundle to Central. It
waits for Central validation but does not publish automatically. Review the
validated deployment and explicitly publish it at
https://central.sonatype.com/publishing/deployments; Maven Central releases are
immutable.
