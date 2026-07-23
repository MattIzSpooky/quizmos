Feature: Choosing a player color
  As a player
  I want to pick a color that fits the cosmos theme
  So that I can tell myself apart from other players on the leaderboard

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Colors"
    And a multiple choice question "Q" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And I create a game for the quiz

  Scenario: Joining with a valid color is honored
    When "Alice" joins the game with color "comet"
    Then "Alice" should be shown to the admin with color "comet"

  Scenario: Joining with an unrecognized color falls back to the default
    When "Alice" joins the game with color "chartreuse"
    Then "Alice" should be shown to the admin with color "nebula"

  Scenario: Joining without a color falls back to the default
    When "Alice" joins the game
    Then "Alice" should be shown to the admin with color "nebula"

  Scenario: A player's chosen color appears on the leaderboard
    Given "Alice" joins the game with color "quasar"
    And "Alice" connects to the game websocket
    And the admin starts the game
    And "Alice" answers "A"
    Then the leaderboard should show "Alice" with color "quasar"
