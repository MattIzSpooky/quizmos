Feature: Reviewing an earlier question
  As a quiz admin
  I want to jump back to any question that's already been asked
  So that I can recap without disturbing where live play actually is

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Three Rounds"
    And a multiple choice question "Round 1" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And a multiple choice question "Round 2" with options:
      | text | correct |
      | A    | false   |
      | B    | true    |
    And a multiple choice question "Round 3" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin starts the game

  Scenario: Hopping back more than one question shows it read-only
    Given "Alice" answers "A"
    And the admin advances to the next question
    And "Alice" answers "B"
    And the admin advances to the next question
    When the admin reviews question 1
    Then "Alice" should receive a "question.reviewed" message
    And the leaderboard should show "Alice" with score 200

  Scenario: Reviewing the current question is allowed
    Given "Alice" answers "A"
    And the admin advances to the next question
    When the admin reviews question 2
    Then "Alice" should receive a "question.reviewed" message

  Scenario: Reviewing a question that hasn't been asked yet is rejected
    Then reviewing question 2 should fail with status 400
