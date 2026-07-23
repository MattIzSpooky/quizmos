Feature: Public game lookup
  As a player
  I want to look up a game by its join code before joining
  So that the app can show the quiz name and confirm the code is real

  Background:
    Given I am authenticated as an admin
    And a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And I create a game for the quiz

  Scenario: Looking up a game by its code shows the quiz title and status
    Then the public game lookup should show quiz "General Knowledge" and status "lobby"

  Scenario: Looking up a game reflects its status once it's live
    Given the admin starts the game
    Then the public game lookup should show quiz "General Knowledge" and status "in_progress"

  Scenario: Looking up an unknown code fails
    Then the public game lookup for code "NOSUCH" should fail with status 404

  Scenario: The public leaderboard reflects scores once the game is live
    Given "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin starts the game
    And "Alice" answers "4"
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    Then the public leaderboard should show "Alice" with score 100
