package dev.castingcode.tuicast;

import java.time.Duration;
import java.util.*;

public record SshConfig(
    String address,
    String username,
    String password,
    String privateKey,
    String privateKeyPassphrase,
    String knownHostsFile,
    String hostKeyFingerprint,
    boolean insecureSkipHostKeyCheck,
    Duration connectTimeout)
    implements ConnectionConfig {
  public static Builder builder(String address, String username) {
    return new Builder(address, username);
  }

  public Map<String, Object> parameters(Duration d) {
    validate();
    var m = new HashMap<String, Object>();
    m.put("protocol", "ssh");
    m.put("address", address);
    m.put("username", username);
    put(m, "password", password);
    put(m, "privateKey", privateKey);
    put(m, "privateKeyPassphrase", privateKeyPassphrase);
    put(m, "knownHostsFile", knownHostsFile);
    put(m, "hostKeyFingerprint", hostKeyFingerprint);
    m.put("insecureSkipHostKeyCheck", insecureSkipHostKeyCheck);
    m.put("connectTimeoutMilliseconds", TelnetConfig.millis(timeout(d)));
    return Map.copyOf(m);
  }

  public Duration timeout(Duration d) {
    return connectTimeout == null ? d : connectTimeout;
  }

  private void validate() {
    if (blank(address) || blank(username))
      throw new IllegalArgumentException("SSH address and username are required");
    if (blank(password) && blank(privateKey))
      throw new IllegalArgumentException("SSH password or private key is required");
    int n =
        (blank(knownHostsFile) ? 0 : 1)
            + (blank(hostKeyFingerprint) ? 0 : 1)
            + (insecureSkipHostKeyCheck ? 1 : 0);
    if (n != 1)
      throw new IllegalArgumentException(
          "exactly one SSH host-key verification option is required");
    TelnetConfig.millis(timeout(Duration.ofSeconds(30)));
  }

  private static boolean blank(String s) {
    return s == null || s.isBlank();
  }

  private static void put(Map<String, Object> m, String k, String v) {
    if (v != null) m.put(k, v);
  }

  public static final class Builder {
    private final String a, u;
    private String p, k, pp, kh, fp;
    private boolean insecure;
    private Duration timeout;

    Builder(String a, String u) {
      this.a = a;
      this.u = u;
    }

    public Builder password(String v) {
      p = v;
      return this;
    }

    public Builder privateKey(String v) {
      k = v;
      return this;
    }

    public Builder privateKeyPassphrase(String v) {
      pp = v;
      return this;
    }

    public Builder knownHostsFile(String v) {
      kh = v;
      return this;
    }

    public Builder hostKeyFingerprint(String v) {
      fp = v;
      return this;
    }

    public Builder insecureSkipHostKeyCheck(boolean v) {
      insecure = v;
      return this;
    }

    public Builder connectTimeout(Duration v) {
      timeout = v;
      return this;
    }

    public SshConfig build() {
      return new SshConfig(a, u, p, k, pp, kh, fp, insecure, timeout);
    }
  }
}
