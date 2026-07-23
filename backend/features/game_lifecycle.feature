Feature: Game lifecycle
  As a quiz admin
  I want to create game sessions from a quiz and let players join
  So that a group can play together with a short join code

  Background:
    Given I am authenticated as an admin
    And a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |

  Scenario: A player joins a game in the lobby
    Given I create a game for the quiz
    When "Alice" joins the game
    Then the request should succeed
    And the game should have 1 players

  Scenario: A second player joining raises the player count
    Given I create a game for the quiz
    When "Alice" joins the game
    And "Bob" joins the game
    Then the game should have 2 players

  Scenario: Joining an unknown game code fails
    When "Alice" tries to join game code "NOSUCH"
    Then the request should fail with status 404

  Scenario: Starting a game with no questions is rejected
    Given I create a quiz titled "Empty quiz"
    When I create a game for the quiz
    Then the request should fail with status 400

  Scenario: Starting an already-started game is rejected
    Given I create a game for the quiz
    And the admin starts the game
    Then starting the game should fail with status 409
