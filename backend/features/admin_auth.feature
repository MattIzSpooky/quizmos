Feature: Admin endpoint authentication and authorization
  As the operator of Quizmos
  I want every admin endpoint to reject anonymous or under-privileged requests
  So that only quiz admins can manage quizzes and run games

  Background:
    Given I am authenticated as an admin
    And a quiz titled "Locked Down"

  # /admin/quizzes goes through the standard OpenAPI security scheme
  # (openapi3filter -> Keycloak.AuthenticationFunc).
  Scenario: Listing quizzes with no bearer token is rejected
    When I request the admin quizzes list with no bearer token
    Then the request should fail with status 401

  Scenario: Listing quizzes with an invalid bearer token is rejected
    When I request the admin quizzes list with an invalid bearer token
    Then the request should fail with status 401

  Scenario: Listing quizzes as a user without the admin role is rejected
    When I request the admin quizzes list as a user without the admin role
    Then the request should fail with status 403

  # The question-media routes skip the standard request validator
  # entirely and check Authorization themselves (see
  # internal/handlers/media.go) — a different code path worth covering
  # on its own.
  Scenario: Uploading question media with no bearer token is rejected
    When I try to upload media with no bearer token
    Then the request should fail with status 401

  Scenario: Uploading question media with an invalid bearer token is rejected
    When I try to upload media with an invalid bearer token
    Then the request should fail with status 401

  Scenario: Uploading question media as a user without the admin role is rejected
    When I try to upload media as a user without the admin role
    Then the request should fail with status 403

  Scenario: Deleting question media with no bearer token is rejected
    When I try to delete media with no bearer token
    Then the request should fail with status 401

  Scenario: Deleting question media with an invalid bearer token is rejected
    When I try to delete media with an invalid bearer token
    Then the request should fail with status 401

  Scenario: Deleting question media as a user without the admin role is rejected
    When I try to delete media as a user without the admin role
    Then the request should fail with status 403
