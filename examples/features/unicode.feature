Feature: Unicode rendering
  The reference TUI renders representative Unicode characters at stable screen
  locations.

  Background:
    Given I am connected to the reference TUI
    And I am logged in

  Scenario: Inspect Unicode samples
    When I select the "Unicode" menu option
    Then the application is on the "Unicode" screen
    And the "CJK" sample renders "漢字" at zero-based column 21 and row 7
    And the "Emoji" sample renders "🙂" at zero-based column 21 and row 8
    And the "ZWJ" sample renders "👩‍💻" at zero-based column 21 and row 9
    And the "Flag" sample renders "🇺🇸" at zero-based column 21 and row 10
    And the "Skin tone" sample renders "👍🏽" at zero-based column 21 and row 11
