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

  Scenario: Joining a game that has already started is rejected
    Given I create a game for the quiz
    And the admin starts the game
    When "Alice" joins the game
    Then the request should fail with status 409
    And the game should have 0 players

  Scenario: Joining a game that has ended is rejected
    Given I create a game for the quiz
    And the admin starts the game
    And the admin ends the game
    When "Alice" joins the game
    Then the request should fail with status 409

  Scenario: Starting a game with no questions is rejected
    Given I create a quiz titled "Empty quiz"
    When I create a game for the quiz
    Then the request should fail with status 400

  Scenario: Starting an already-started game is rejected
    Given I create a game for the quiz
    And the admin starts the game
    Then starting the game should fail with status 409

  Scenario: Kicking a player removes them from the lobby
    Given I create a game for the quiz
    And "Alice" joins the game
    And "Bob" joins the game
    When the admin kicks "Bob"
    Then the game should have 1 players

  Scenario: A kicked player can rejoin
    Given I create a game for the quiz
    And "Alice" joins the game
    And the admin kicks "Alice"
    When "Alice" rejoins the game
    Then the request should succeed
    And the game should have 1 players

  Scenario: Kicking is not allowed once the game has started
    Given I create a game for the quiz
    And "Alice" joins the game
    And the admin starts the game
    Then kicking "Alice" should fail with status 409

  Scenario: The kicked player is notified over their own connection
    Given I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    When the admin kicks "Alice"
    Then "Alice" should receive a "player.kicked" message

  Scenario: A player who joined before the game started can still connect once it's live
    Given I create a game for the quiz
    And "Alice" joins the game
    And the admin starts the game
    When "Alice" connects to the game websocket
    Then "Alice" should receive a "question.started" message

  Scenario: Rejoining while still in the lobby just updates the player, not a duplicate
    Given I create a game for the quiz
    And "Alice" joins the game
    When "Alice" rejoins the game
    Then the request should succeed
    And the game should have 1 players

  Scenario: Kicking a player who never joined fails
    Given I create a game for the quiz
    Then kicking a player who never joined should fail with status 404

  Scenario: The admin sees a player as connected once their websocket is open
    Given I create a game for the quiz
    And "Alice" joins the game
    Then the admin should see "Alice" as not connected
    When "Alice" connects to the game websocket
    Then the admin should see "Alice" as connected

  Scenario: Creating a game for a quiz that doesn't exist fails
    When I try to create a game for an unknown quiz
    Then the request should fail with status 404
