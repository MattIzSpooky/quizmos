Feature: Question media
  As a quiz admin
  I want to attach an optional image or audio fragment to a question
  So that players see it when the question comes up, and I can reference it live

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Picture Round"
    And a multiple choice question "Name this landmark" with options:
      | text  | correct |
      | Paris | true    |
      | Rome  | false   |

  Scenario: Uploading an image attaches it to the question
    When the admin uploads an image as media for "Name this landmark"
    Then "Name this landmark" should have image media

  Scenario: Uploading audio attaches it to the question
    When the admin uploads an audio fragment as media for "Name this landmark"
    Then "Name this landmark" should have audio media

  Scenario: Uploading new media replaces the old media
    Given the admin uploads an image as media for "Name this landmark"
    When the admin uploads an audio fragment as media for "Name this landmark"
    Then "Name this landmark" should have audio media

  Scenario: Removing media clears it
    Given the admin uploads an image as media for "Name this landmark"
    When the admin removes the media for "Name this landmark"
    Then "Name this landmark" should have no media

  Scenario: An oversized image is rejected
    Then uploading an oversized image for "Name this landmark" should fail with status 400

  Scenario: An unsupported media type is rejected
    Then uploading an unsupported media type for "Name this landmark" should fail with status 400

  Scenario: A player sees the question's image when it starts
    Given the admin uploads an image as media for "Name this landmark"
    And I create a game for the quiz
    And "Alice" joins the game
    And "Alice" connects to the game websocket
    When the admin starts the game
    Then "Alice" should receive a "question.started" message with image media

  Scenario: Removing media that was never attached is a harmless no-op
    When the admin removes the media for "Name this landmark"
    Then "Name this landmark" should have no media

  Scenario: Uploading media for a question that doesn't exist fails
    Then uploading media for an unknown question should fail with status 404
