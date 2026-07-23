Feature: Live gameplay over the websocket
  As a player
  I want to answer questions in real time and see the leaderboard update
  So that the quiz feels live even though most of the API is plain REST

  Background:
    Given I am authenticated as an admin
    And a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And a multiple choice question "Capital of France?" with options:
      | text   | correct |
      | Paris  | true    |
      | Berlin | false   |
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket

  Scenario: A player receives the first question when the game starts
    When the admin starts the game
    Then "Alice" should receive a "game.started" message
    And "Alice" should receive a "question.started" message

  Scenario: A correct answer is scored and reflected on the leaderboard
    Given the admin starts the game
    When "Alice" answers "4"
    Then "Alice" should receive an "answer.result" message with correct true and 100 points
    When the admin advances to the next question
    Then the leaderboard should show "Alice" with score 100

  Scenario: An incorrect answer scores zero points
    Given the admin starts the game
    When "Alice" answers "3"
    Then "Alice" should receive an "answer.result" message with correct false and 0 points

  Scenario: The game ends after the last question and broadcasts final results
    Given the admin starts the game
    And "Alice" answers "4"
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    When the admin advances to the next question
    And "Alice" answers "Paris"
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    And the admin advances to the next question
    Then "Alice" should receive a "game.ended" message
    And the leaderboard should show "Alice" with score 200
