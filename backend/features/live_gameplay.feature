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

  Scenario: Going back re-shows the previous question without changing scores
    Given the admin starts the game
    And "Alice" answers "4"
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    And the admin advances to the next question
    When the admin goes back to the previous question
    Then "Alice" should receive a "question.reviewed" message
    And the leaderboard should show "Alice" with score 100

  Scenario: Going back past the first question is rejected
    Given the admin starts the game
    Then going back should fail with status 400

  Scenario: Answering the same question twice is rejected
    Given the admin starts the game
    And "Alice" answers "4"
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    When "Alice" answers "3"
    Then "Alice" should receive an "error" message

  Scenario: Answering with a question id that isn't the live question is rejected
    Given the admin starts the game
    When "Alice" answers with a mismatched question id
    Then "Alice" should receive an "error" message

  Scenario: Answering with an option that doesn't exist is rejected
    Given the admin starts the game
    When "Alice" answers with an option that doesn't exist
    Then "Alice" should receive an "error" message

  Scenario: Submitting free text to a multiple choice question is rejected
    Given the admin starts the game
    When "Alice" submits free text on a multiple choice question
    Then "Alice" should receive an "error" message

  Scenario: The admin can force-end a game before the last question
    Given the admin starts the game
    When the admin ends the game
    Then "Alice" should receive a "game.ended" message

  Scenario: A second player connecting is announced to those already in the room
    When "Bob" joins the game
    And "Bob" connects to the game websocket
    Then "Alice" should receive a "presence.playerJoined" message

  Scenario: A player disconnecting is announced to those still in the room
    Given "Bob" joins the game
    And "Bob" connects to the game websocket
    And "Alice" should receive a "presence.playerJoined" message
    When "Bob" disconnects
    Then "Alice" should receive a "presence.playerLeft" message

  Scenario: Advancing broadcasts the ended question's correct answer and response count
    Given the admin starts the game
    And "Alice" answers "4"
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    When the admin advances to the next question
    Then "Alice" should receive a "question.ended" message with 1 correct response

  Scenario: Sending an unrecognized message type is rejected
    Given the admin starts the game
    When "Alice" sends a message of unknown type
    Then "Alice" should receive an "error" message

  Scenario: Sending a malformed answer.submit payload is rejected
    Given the admin starts the game
    When "Alice" sends a malformed answer.submit payload
    Then "Alice" should receive an "error" message

  Scenario: Answering before the game has started is rejected
    When "Alice" answers before the game starts
    Then "Alice" should receive an "error" message
