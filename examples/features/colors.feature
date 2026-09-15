Feature: Colors and attributes
  The reference TUI renders deterministic indexed colors and text attributes.

  Background:
    Given I am connected to the reference TUI
    And I am logged in

  Scenario: Inspect colors and text attributes
    When I select the "Colors and Attributes" menu option
    Then the application is on the "Colors and Attributes" screen
    And "ANSI 8 COLORS (foreground)" begins at zero-based column 0 and row 4
    And "BOLD" is rendered with the "bold" attribute
    And "UNDERLINE" is rendered with the "underline" attribute
    And the "196" color sample uses foreground 15 and background 196
