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
