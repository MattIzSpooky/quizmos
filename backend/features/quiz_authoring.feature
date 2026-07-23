Feature: Quiz authoring
  As a quiz admin
  I want to create quizzes with multiple-choice questions
  So that I can run live games from them

  Background:
    Given I am authenticated as an admin

  Scenario: Creating a quiz starts with no questions
    When I create a quiz titled "General Knowledge"
    Then the quiz should have 0 questions

  Scenario: Adding a multiple choice question
    Given a quiz titled "General Knowledge"
    When I add a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    Then the quiz should have 1 questions

  Scenario: Adding several questions
    Given a quiz titled "General Knowledge"
    When I add a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And I add a multiple choice question "Capital of France?" with options:
      | text   | correct |
      | Paris  | true    |
      | Berlin | false   |
    Then the quiz should have 2 questions

  Scenario: Adding a free text question
    Given a quiz titled "General Knowledge"
    When I add a free text question "Name a mammal"
    Then the quiz should have 1 questions

  Scenario: A free text question cannot have options
    Given a quiz titled "General Knowledge"
    When I try to add a free text question "Name a mammal" with options:
      | text | correct |
      | Dog  | true    |
    Then the request should fail with status 400

  Scenario: A multiple choice question needs at least two options
    Given a quiz titled "General Knowledge"
    When I try to add a multiple choice question "Only one option?" with options:
      | text | correct |
      | Only | true    |
    Then the request should fail with status 400

  Scenario: Updating a question's prompt and points
    Given a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    When I update "What is 2 + 2?" to prompt "What is 2 plus 2?" and 500 points
    Then "What is 2 plus 2?" should have 500 points

  Scenario: Deleting a question removes it from the quiz
    Given a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    When I delete the question "What is 2 + 2?"
    Then the quiz should have 0 questions

  Scenario: Reordering a quiz's questions
    Given a quiz titled "General Knowledge"
    And a multiple choice question "First" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    And a multiple choice question "Second" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    When I reorder the questions to:
      | Second |
      | First  |
    Then the request should succeed
    And the questions should be in this order:
      | Second |
      | First  |

  Scenario: Reordering with a question id that doesn't belong to the quiz is rejected
    Given a quiz titled "General Knowledge"
    And a multiple choice question "First" with options:
      | text | correct |
      | A    | true    |
      | B    | false   |
    When I try to reorder the questions to:
      | First  |
      | Ghost  |
    Then the request should fail with status 400

  Scenario: Updating a quiz's title and timed toggle
    Given a quiz titled "Old Name"
    When I update the quiz to title "New Name" and timed false
    Then the quiz should be titled "New Name" and untimed

  Scenario: Deleting a quiz with no games removes it
    Given a quiz titled "Throwaway"
    When I delete the quiz
    Then getting the quiz should fail with status 404

  Scenario: Deleting a quiz also removes its games, players, and media
    Given a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And the admin uploads an image as media for "What is 2 + 2?"
    And I create a game for the quiz
    And "Alice" joins the game
    When I delete the quiz
    Then getting the quiz should fail with status 404
    And getting the game should fail with status 404
    And the previously uploaded media should no longer be reachable

  Scenario: Getting a quiz that doesn't exist fails
    Then getting an unknown quiz should fail with status 404

  Scenario: Listing quizzes includes ones I've created
    Given a quiz titled "Alpha Quiz"
    And a quiz titled "Beta Quiz"
    Then the quiz list should include "Alpha Quiz" and "Beta Quiz"

  Scenario: Listing games includes the one I created
    Given a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And I create a game for the quiz
    Then the game list should include this game

  Scenario: Listing games can be filtered by status
    Given a quiz titled "General Knowledge"
    And a multiple choice question "What is 2 + 2?" with options:
      | text | correct |
      | 3    | false   |
      | 4    | true    |
    And I create a game for the quiz
    And the admin starts the game
    Then the game list filtered by status "in_progress" should include this game
    And the game list filtered by status "lobby" should not include this game
