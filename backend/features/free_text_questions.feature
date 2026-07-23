Feature: Free-text questions
  As a quiz admin
  I want to ask open-ended questions that I grade myself
  So that I'm not limited to multiple choice

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Open Round"
    And a free text question "Name a primary color"
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin starts the game

  Scenario: Submitting a free-text answer is pending until graded
    When "Alice" submits the free-text answer "Red"
    Then "Alice" should receive a pending "answer.result" message
    And the leaderboard should show "Alice" with score 0

  Scenario: Grading an answer as correct awards full points
    Given "Alice" submits the free-text answer "Red"
    And "Alice" should receive a pending "answer.result" message
    When the admin grades "Alice"'s answer to "Name a primary color" as correct
    Then "Alice" should receive an "answer.result" message with correct true and 100 points
    And the leaderboard should show "Alice" with score 100

  Scenario: Grading an answer as incorrect awards no points
    Given "Alice" submits the free-text answer "Turquoise"
    And "Alice" should receive a pending "answer.result" message
    When the admin grades "Alice"'s answer to "Name a primary color" as incorrect
    Then "Alice" should receive an "answer.result" message with correct false and 0 points
    And the leaderboard should show "Alice" with score 0

  Scenario: Re-grading corrects the player's score by the difference, not by adding again
    Given "Alice" submits the free-text answer "Red"
    And "Alice" should receive a pending "answer.result" message
    And the admin grades "Alice"'s answer to "Name a primary color" as correct
    And the leaderboard should show "Alice" with score 100
    When the admin grades "Alice"'s answer to "Name a primary color" as incorrect
    Then the leaderboard should show "Alice" with score 0

  Scenario: An empty free-text answer is rejected
    When "Alice" submits the free-text answer ""
    Then "Alice" should receive an "error" message

  Scenario: A free-text answer over 500 characters is rejected
    When "Alice" submits an over-length free-text answer
    Then "Alice" should receive an "error" message
