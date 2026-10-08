Feature: Login
  The reference TUI accepts its configured application credentials and keeps
  unauthenticated users on the login screen. The password is read from the
  TUICAST_REFERENCE_APP_PASSWORD environment variable rather than written here.

  Background:
    Given I am connected to the reference TUI

  Scenario: Successful login
    When I enter username "operator"
    And I enter the configured password
    And I press Enter
    Then the application is on the "main menu" screen
    And the screen contains "Authenticated as operator"

  Scenario: Unsuccessful login
    When I enter username "unknown"
    And I enter password "incorrect"
    And I press Enter
    Then the application is on the "login" screen
    And the application reports that authentication failed
