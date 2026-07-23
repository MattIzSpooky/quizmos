Feature: Resetting a question's answers
  As a quiz admin
  I want to wipe everyone's answer to a question and reverse its points
  So that I can recover from a mistake without abandoning the game

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Two Rounds"
    And a multiple choice question "Round 1" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And a multiple choice question "Round 2" with options:
      | text | correct |
      | A    | false   |
      | B    | true    |
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin starts the game

  Scenario: Resetting the current question lets a player answer it again
    Given "Alice" answers "A"
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    And the leaderboard should show "Alice" with score 100
    When the admin resets the answers for question 1
    Then "Alice" should receive a "question.answersReset" message
    And the leaderboard should show "Alice" with score 0
    And "Alice" answers "A" again
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    And the leaderboard should show "Alice" with score 100

  Scenario: Resetting a question nobody answered is a harmless no-op
    When the admin resets the answers for question 1
    Then "Alice" should receive a "question.answersReset" message
    And the leaderboard should show "Alice" with score 0

  Scenario: Resetting a question that hasn't been asked yet is rejected
    Then resetting answers for question 2 should fail with status 400

  Scenario: Resetting answers only works while the game is in progress
    Given "Alice" answers "A"
    And the admin advances to the next question
    And "Alice" answers "B"
    And the admin advances to the next question
    Then resetting answers for question 1 should fail with status 409
