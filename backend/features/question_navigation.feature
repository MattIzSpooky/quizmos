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

  Scenario: Reviewing the current (live) question resumes live play instead of a recap
    Given "Alice" answers "A"
    And the admin advances to the next question
    When the admin reviews question 2
    Then "Alice" should receive a "question.started" message

  Scenario: Switching back to the latest question un-reviews it and continues the game
    Given "Alice" answers "A"
    And the admin advances to the next question
    And the admin reviews question 1
    And "Alice" should receive a "question.reviewed" message
    When the admin reviews question 2
    Then "Alice" should receive a "question.started" message
    And "Alice" answers "B"
    And "Alice" should receive an "answer.result" message with correct true and 100 points

  Scenario: Reviewing a question that hasn't been asked yet is rejected
    Then reviewing question 2 should fail with status 400

  Scenario: A player who reconnects mid-recap sees the recap, not live play
    Given "Alice" answers "A"
    And the admin advances to the next question
    And "Alice" answers "B"
    And the admin advances to the next question
    And the admin reviews question 1
    And "Alice" should receive a "question.reviewed" message
    When "Alice" disconnects
    And "Alice" reconnects to the game websocket
    Then "Alice" should receive a "question.reviewed" message

  Scenario: Reviewing is rejected while the game is still in the lobby
    Given a quiz titled "Not Started Yet"
    And a multiple choice question "Q" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And I create a game for the quiz
    Then reviewing question 1 should fail with status 409

  Scenario: Reviewing is rejected once the game has ended
    Given a quiz titled "Already Over"
    And a multiple choice question "Q" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And I create a game for the quiz
    And the admin starts the game
    And the admin ends the game
    Then reviewing question 1 should fail with status 409
