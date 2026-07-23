Feature: Per-quiz timing toggle
  As a quiz admin
  I want to turn the countdown off for a quiz
  So that a game doesn't have to feel like Kahoot

  Background:
    Given I am authenticated as an admin

  Scenario: Players see a countdown for a timed quiz
    Given a quiz titled "Timed Quiz"
    And a multiple choice question "2+2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    When the admin starts the game
    Then "Alice" should receive a "question.started" message with timed true

  Scenario: Players see no countdown for an untimed quiz
    Given an untimed quiz titled "Relaxed Quiz"
    And a multiple choice question "2+2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    When the admin starts the game
    Then "Alice" should receive a "question.started" message with timed false
