Feature: Websocket connection authorization
  As the operator of Quizmos
  I want the live-gameplay websocket to refuse anyone who hasn't actually joined
  So that the play channel can't be reached just by guessing a game code

  Background:
    Given I am authenticated as an admin
    And a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And I create a game for the quiz

  Scenario: Connecting without ever joining is rejected
    When "Alice" tries to connect to the game websocket without joining
    Then the websocket connection should be rejected with status 403

  Scenario: Connecting with a malformed client id is rejected
    When someone tries to connect to the game websocket with client id "not-a-uuid"
    Then the websocket connection should be rejected with status 400
