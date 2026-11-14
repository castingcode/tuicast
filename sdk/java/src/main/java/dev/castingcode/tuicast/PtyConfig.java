package dev.castingcode.tuicast;

import java.time.Duration;
import java.util.*;

/**
 * Runs a program on a pseudo-terminal on the driver's machine. Each session opened on the
 * connection starts its own instance of the program, with {@code TERM} set to the session's
 * terminal profile. A command without a path separator is looked up on the driver's {@code PATH}; a
 * relative path is resolved from the working directory. The program inherits the driver's
 * environment plus {@code environment} unless {@code clearEnvironment} is true. {@code TERM} cannot
 * be set.
 */
public record PtyConfig(
    String command,
    List<String> arguments,
    String workingDirectory,
    Map<String, String> environment,
    boolean clearEnvironment,
    Duration connectTimeout)
    implements ConnectionConfig {
  public PtyConfig {
    arguments = arguments == null ? List.of() : List.copyOf(arguments);
    environment = environment == null ? Map.of() : Map.copyOf(environment);
  }

  public PtyConfig(String command) {
    this(command, null, null, null, false, null);
  }

  public static Builder builder(String command) {
    return new Builder(command);
  }

  public Map<String, Object> parameters(Duration d) {
    if (command == null || command.isBlank())
      throw new IllegalArgumentException("PTY command is required");
    var m = new HashMap<String, Object>();
    m.put("protocol", "pty");
    m.put("command", command);
    m.put("arguments", arguments);
    if (workingDirectory != null) m.put("workingDirectory", workingDirectory);
    m.put("environment", environment);
    m.put("clearEnvironment", clearEnvironment);
    m.put("connectTimeoutMilliseconds", TelnetConfig.millis(timeout(d)));
    return Map.copyOf(m);
  }

  public Duration timeout(Duration d) {
    return connectTimeout == null ? d : connectTimeout;
  }

  public static final class Builder {
    private final String command;
    private final List<String> arguments = new ArrayList<>();
    private final Map<String, String> environment = new HashMap<>();
    private String workingDirectory;
    private boolean clearEnvironment;
    private Duration timeout;

    Builder(String command) {
      this.command = command;
    }

    public Builder arguments(String... values) {
      arguments.addAll(List.of(values));
      return this;
    }

    public Builder workingDirectory(String v) {
      workingDirectory = v;
      return this;
    }

    public Builder environment(String name, String value) {
      environment.put(name, value);
      return this;
    }

    public Builder clearEnvironment(boolean v) {
      clearEnvironment = v;
      return this;
    }

    public Builder connectTimeout(Duration v) {
      timeout = v;
      return this;
    }

    public PtyConfig build() {
      return new PtyConfig(
          command, arguments, workingDirectory, environment, clearEnvironment, timeout);
    }
  }
}
