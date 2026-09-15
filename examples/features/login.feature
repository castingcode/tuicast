Feature: Login
  The reference TUI accepts its documented application credentials and keeps
  unauthenticated users on the login screen.

  Background:
    Given I am connected to the reference TUI

  Scenario: Successful login
    When I enter username "operator"
    And I enter password "casting"
    And I press Enter
    Then the application is on the "main menu" screen
    And the screen contains "Authenticated as operator"

  Scenario: Unsuccessful login
    When I enter username "unknown"
    And I enter password "incorrect"
    And I press Enter
    Then the application is on the "login" screen
    And the application reports that authentication failed
