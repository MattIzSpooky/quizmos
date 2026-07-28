Feature: Admin live game-control websocket
  As the operator of Quizmos
  I want the admin live-control page's websocket to require a fresh, single-use ticket from an authenticated admin
  So that only someone with a valid admin session can watch a game's live state, and browsers never have to put a bearer token in a URL

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Control Room"
    And a free text question "Capital of France"
    And I create a game for the quiz

  Scenario: Minting a websocket ticket with no bearer token is rejected
    When I try to mint an admin websocket ticket with no bearer token
    Then the request should fail with status 401

  Scenario: Minting a websocket ticket with an invalid bearer token is rejected
    When I try to mint an admin websocket ticket with an invalid bearer token
    Then the request should fail with status 401

  Scenario: Minting a websocket ticket as a user without the admin role is rejected
    When I try to mint an admin websocket ticket as a user without the admin role
    Then the request should fail with status 403

  Scenario: Connecting without a ticket is rejected
    When someone tries to connect to the admin game control websocket without a ticket
    Then the websocket connection should be rejected with status 401

  Scenario: Connecting with an already-used ticket is rejected
    Given the admin mints a websocket ticket
    And the admin connects to the game control websocket using that ticket
    When someone tries to reuse that same ticket to connect
    Then the websocket connection should be rejected with status 401

  Scenario: Connecting with a valid ticket receives the game's live broadcasts
    Given the admin connects to the game control websocket
    When the admin starts the game
    Then the admin should receive a "game.started" message
    And the admin should receive a "question.started" message

  Scenario: A free-text submission is pushed to the connected admin
    Given "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin connects to the game control websocket
    And the admin starts the game
    When "Alice" submits the free-text answer "Paris"
    Then the admin should receive a "freeTextAnswer.updated" message

  Scenario: Grading a free-text answer is pushed to the connected admin
    Given "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin connects to the game control websocket
    And the admin starts the game
    And "Alice" submits the free-text answer "Paris"
    And the admin should receive a "freeTextAnswer.updated" message
    When the admin grades "Alice"'s answer to "Capital of France" as correct
    Then the admin should receive a "freeTextAnswer.updated" message

  Scenario: Minting a websocket ticket for an unknown game fails
    When I try to mint an admin websocket ticket for an unknown game
    Then the request should fail with status 404

  Scenario: The admin's websocket is closed once the game ends
    Given the admin connects to the game control websocket
    When the admin ends the game
    Then the admin's websocket connection should be closed

  Scenario: Resuming live play after a review reaches a connected admin
    Given I am authenticated as an admin
    And a quiz titled "Two Questions"
    And a multiple choice question "Round 1" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And a multiple choice question "Round 2" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And I create a game for the quiz
    And the admin connects to the game control websocket
    And the admin starts the game
    And the admin should receive a "question.started" message
    And the admin advances to the next question
    And the admin should receive a "question.started" message
    And the admin reviews question 1
    When the admin reviews question 2
    Then the admin should receive a "question.started" message

  Scenario: Grading a free-text answer also reaches a second connected admin tab
    Given "Alice" joins the game
    And "Alice" connects to the game websocket
    And the admin connects to the game control websocket
    And a second admin tab connects to the game control websocket
    And the admin starts the game
    And "Alice" submits the free-text answer "Paris"
    And the second admin tab should receive a "freeTextAnswer.updated" message
    When the admin grades "Alice"'s answer to "Capital of France" as correct
    Then the second admin tab should receive a "freeTextAnswer.updated" message
