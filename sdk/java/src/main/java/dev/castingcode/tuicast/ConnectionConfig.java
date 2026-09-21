package dev.castingcode.tuicast;

import java.time.Duration;
import java.util.Map;

public sealed interface ConnectionConfig permits TelnetConfig, SshConfig {
  Map<String, Object> parameters(Duration defaultTimeout);

  Duration timeout(Duration defaultTimeout);
}
