Feature: Function keys
  The reference TUI reports named keys and modifiers as they are received.

  Background:
    Given I am connected to the reference TUI
    And I am logged in
    When I select the "Function Keys" menu option
    Then the application is on the "Function Keys" screen

  Scenario: Press a function key
    When I press F5
    Then the captured key count is 1
    And the latest key is "f5"
    And the latest key modifiers are "none"

  Scenario: Press a modified key
    When I press Control+C
    Then the captured key count is 1
    And the latest key is "ctrl+c"
    And the latest key modifiers are "control"
