Feature: Resuming live play after a review preserves the player's own answer
  As a player
  I want my already-submitted answer to still be reflected
  So that reviewing another question and resuming doesn't invite me to answer again

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Two Rounds"
    And a multiple choice question "Round 1" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And a free text question "Round 2"
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin starts the game
    And the admin advances to the next question

  Scenario: Resuming preserves a still-pending free-text answer
    Given "Alice" submits the free-text answer "Some answer"
    And "Alice" should receive a pending "answer.result" message
    And the admin reviews question 1
    And "Alice" should receive a "question.reviewed" message
    When the admin reviews question 2
    Then "Alice" should receive a "question.started" message with your answer pending

  Scenario: Resuming preserves an already-graded free-text answer
    Given "Alice" submits the free-text answer "Some answer"
    And "Alice" should receive a pending "answer.result" message
    And the admin grades "Alice"'s answer to "Round 2" as correct
    And "Alice" should receive an "answer.result" message with correct true and 100 points
    And the admin reviews question 1
    And "Alice" should receive a "question.reviewed" message
    When the admin reviews question 2
    Then "Alice" should receive a "question.started" message with your answer graded correct and 100 points

  Scenario: A player reconnecting mid-question also sees their own pending answer
    Given "Alice" submits the free-text answer "Some answer"
    And "Alice" should receive a pending "answer.result" message
    When "Alice" disconnects
    And "Alice" reconnects to the game websocket
    Then "Alice" should receive a "question.started" message with your answer pending
