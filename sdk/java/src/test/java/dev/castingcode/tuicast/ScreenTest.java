package dev.castingcode.tuicast;

import static org.junit.jupiter.api.Assertions.*;

import com.fasterxml.jackson.databind.JsonNode;
import java.nio.file.*;
import org.junit.jupiter.api.Test;

class ScreenTest {
  @Test
  void sharedFixtures() throws Exception {
    JsonNode root =
        Driver.JSON.readTree(
            Files.readString(Path.of("../../schema/testdata/screen-queries.json")));
    for (var fixture : root.get("cases")) {
      Screen s = Driver.JSON.treeToValue(fixture.get("screen"), Screen.class);
      assertTrue(s.contains(fixture.get("contains").asText()));
      assertEquals(
          Driver.JSON.treeToValue(fixture.get("position"), Screen.Position.class),
          s.find(fixture.get("find").asText()).orElseThrow());
      assertFalse(s.find("MISSING").isPresent());
      assertFalse(s.line(s.find(fixture.get("find").asText()).orElseThrow().row()).isEmpty());
    }
  }
}
