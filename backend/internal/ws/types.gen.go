// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    answerCount, err := UnmarshalAnswerCount(bytes)
//    bytes, err = answerCount.Marshal()
//
//    answerResult, err := UnmarshalAnswerResult(bytes)
//    bytes, err = answerResult.Marshal()
//
//    answerSubmit, err := UnmarshalAnswerSubmit(bytes)
//    bytes, err = answerSubmit.Marshal()
//
//    errorPayload, err := UnmarshalErrorPayload(bytes)
//    bytes, err = errorPayload.Marshal()
//
//    gameEnded, err := UnmarshalGameEnded(bytes)
//    bytes, err = gameEnded.Marshal()
//
//    gameStarted, err := UnmarshalGameStarted(bytes)
//    bytes, err = gameStarted.Marshal()
//
//    leaderboardEntry, err := UnmarshalLeaderboardEntry(bytes)
//    bytes, err = leaderboardEntry.Marshal()
//
//    leaderboardUpdated, err := UnmarshalLeaderboardUpdated(bytes)
//    bytes, err = leaderboardUpdated.Marshal()
//
//    playerKicked, err := UnmarshalPlayerKicked(bytes)
//    bytes, err = playerKicked.Marshal()
//
//    playerSummary, err := UnmarshalPlayerSummary(bytes)
//    bytes, err = playerSummary.Marshal()
//
//    presencePlayerJoined, err := UnmarshalPresencePlayerJoined(bytes)
//    bytes, err = presencePlayerJoined.Marshal()
//
//    presencePlayerLeft, err := UnmarshalPresencePlayerLeft(bytes)
//    bytes, err = presencePlayerLeft.Marshal()
//
//    questionAnswersReset, err := UnmarshalQuestionAnswersReset(bytes)
//    bytes, err = questionAnswersReset.Marshal()
//
//    questionEnded, err := UnmarshalQuestionEnded(bytes)
//    bytes, err = questionEnded.Marshal()
//
//    questionOption, err := UnmarshalQuestionOption(bytes)
//    bytes, err = questionOption.Marshal()
//
//    questionReviewed, err := UnmarshalQuestionReviewed(bytes)
//    bytes, err = questionReviewed.Marshal()
//
//    questionStarted, err := UnmarshalQuestionStarted(bytes)
//    bytes, err = questionStarted.Marshal()
//
//    yourAnswer, err := UnmarshalYourAnswer(bytes)
//    bytes, err = yourAnswer.Marshal()

package ws

import "time"

import "encoding/json"

func UnmarshalAnswerCount(data []byte) (AnswerCount, error) {
	var r AnswerCount
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AnswerCount) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAnswerResult(data []byte) (AnswerResult, error) {
	var r AnswerResult
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AnswerResult) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalAnswerSubmit(data []byte) (AnswerSubmit, error) {
	var r AnswerSubmit
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *AnswerSubmit) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalErrorPayload(data []byte) (ErrorPayload, error) {
	var r ErrorPayload
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *ErrorPayload) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGameEnded(data []byte) (GameEnded, error) {
	var r GameEnded
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GameEnded) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalGameStarted(data []byte) (GameStarted, error) {
	var r GameStarted
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *GameStarted) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLeaderboardEntry(data []byte) (LeaderboardEntry, error) {
	var r LeaderboardEntry
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LeaderboardEntry) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalLeaderboardUpdated(data []byte) (LeaderboardUpdated, error) {
	var r LeaderboardUpdated
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *LeaderboardUpdated) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPlayerKicked(data []byte) (PlayerKicked, error) {
	var r PlayerKicked
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PlayerKicked) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPlayerSummary(data []byte) (PlayerSummary, error) {
	var r PlayerSummary
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PlayerSummary) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPresencePlayerJoined(data []byte) (PresencePlayerJoined, error) {
	var r PresencePlayerJoined
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PresencePlayerJoined) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalPresencePlayerLeft(data []byte) (PresencePlayerLeft, error) {
	var r PresencePlayerLeft
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *PresencePlayerLeft) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalQuestionAnswersReset(data []byte) (QuestionAnswersReset, error) {
	var r QuestionAnswersReset
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *QuestionAnswersReset) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalQuestionEnded(data []byte) (QuestionEnded, error) {
	var r QuestionEnded
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *QuestionEnded) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalQuestionOption(data []byte) (QuestionOption, error) {
	var r QuestionOption
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *QuestionOption) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalQuestionReviewed(data []byte) (QuestionReviewed, error) {
	var r QuestionReviewed
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *QuestionReviewed) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalQuestionStarted(data []byte) (QuestionStarted, error) {
	var r QuestionStarted
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *QuestionStarted) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

func UnmarshalYourAnswer(data []byte) (YourAnswer, error) {
	var r YourAnswer
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *YourAnswer) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Sent once when a multiple_choice answer is scored automatically. For free_text, sent
// twice: first right after submission with pending true (correct/pointsAwarded not yet
// meaningful), then again once the admin grades it by hand with pending false and the final
// verdict.
type AnswerResult struct {
	Correct       bool   `json:"correct"`
	Pending       bool   `json:"pending"`
	PointsAwarded int64  `json:"pointsAwarded"`
	QuestionID    string `json:"questionId"`
	TotalScore    int64  `json:"totalScore"`
}

// Exactly one of optionId (multiple_choice) or text (free_text, max 500 characters) must be
// set, matching the current question's type.
type AnswerSubmit struct {
	OptionID   *string `json:"optionId,omitempty"`
	QuestionID string  `json:"questionId"`
	Text       *string `json:"text,omitempty"`
}

type ErrorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type GameEnded struct {
	EndedAt          time.Time          `json:"endedAt"`
	FinalLeaderboard []LeaderboardEntry `json:"finalLeaderboard"`
}

type LeaderboardEntry struct {
	ClientID                                                                                 string `json:"clientId"`
	// One of a small, curated set of cosmos-themed color IDs (see the REST API's PlayerColor       
	// schema) — not a full color range.                                                            
	Color                                                                                    Color  `json:"color"`
	Nickname                                                                                 string `json:"nickname"`
	Rank                                                                                     int64  `json:"rank"`
	Score                                                                                    int64  `json:"score"`
}

type GameStarted struct {
	StartedAt time.Time `json:"startedAt"`
}

type LeaderboardUpdated struct {
	Entries       []LeaderboardEntry `json:"entries"`
	QuestionIndex int64              `json:"questionIndex"`
}

// Unicast to the removed player only; the rest of the room instead sees the normal
// presence.playerLeft once the connection drops.
type PlayerKicked struct {
	Reason string `json:"reason"`
}

type PresencePlayerJoined struct {
	Player      PlayerSummary `json:"player"`
	PlayerCount int64         `json:"playerCount"`
}

type PlayerSummary struct {
	ClientID string `json:"clientId"`
	Nickname string `json:"nickname"`
}

type PresencePlayerLeft struct {
	ClientID    string `json:"clientId"`
	PlayerCount int64  `json:"playerCount"`
}

// Broadcast after the admin wipes every answer to a question and reverses its points. If
// this is the question currently live for a client, they should be allowed to answer it
// again.
type QuestionAnswersReset struct {
	QuestionID    string `json:"questionId"`
	QuestionIndex int64  `json:"questionIndex"`
}

// correctOptionId is only present for multiple_choice questions — free_text questions have
// no automatic verdict, so answerCounts is always empty for them too. A free_text player
// instead receives a second answer.result once the admin grades their answer by hand.
type QuestionEnded struct {
	AnswerCounts    []AnswerCount `json:"answerCounts"`
	CorrectOptionID *string       `json:"correctOptionId,omitempty"`
	QuestionID      string        `json:"questionId"`
	QuestionIndex   int64         `json:"questionIndex"`
}

type AnswerCount struct {
	Count    int64  `json:"count"`
	OptionID string `json:"optionId"`
}

// A read-only recap of a previous question, sent when the admin goes back. Unlike
// question.started, the correct answer is already known (it was already revealed), and
// clients must not offer a way to answer it. correctOptionId and answerCounts are only
// meaningful for multiple_choice — free_text has no per-option breakdown.
type QuestionReviewed struct {
	AnswerCounts    []AnswerCount    `json:"answerCounts"`
	CorrectOptionID *string          `json:"correctOptionId,omitempty"`
	Options         []QuestionOption `json:"options"`
	Prompt          string           `json:"prompt"`
	QuestionID      string           `json:"questionId"`
	QuestionIndex   int64            `json:"questionIndex"`
	TotalQuestions  int64            `json:"totalQuestions"`
}

type QuestionOption struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type QuestionStarted struct {
	Options                                                                                     []QuestionOption `json:"options"`
	Prompt                                                                                      string           `json:"prompt"`
	QuestionID                                                                                  string           `json:"questionId"`
	QuestionIndex                                                                               int64            `json:"questionIndex"`
	// Whether the client should show a countdown for this question.                                             
	Timed                                                                                       bool             `json:"timed"`
	TimeLimitSeconds                                                                            int64            `json:"timeLimitSeconds"`
	TotalQuestions                                                                              int64            `json:"totalQuestions"`
	// Determines how the client should collect an answer: option buttons for multiple_choice, a                 
	// text field (max 500 characters) for free_text. free_text questions always have an empty                   
	// options array.                                                                                            
	Type                                                                                        Type             `json:"type"`
	YourAnswer                                                                                  *YourAnswer      `json:"yourAnswer,omitempty"`
}

// Present on question.started only when the recipient has already answered this question —
// this happens when the admin resumes live play after reviewing an earlier question, or
// when a player reconnects mid-question, both of which redeliver question.started for a
// question that may not actually be fresh to this client. Its absence means the client has
// not answered yet.
type YourAnswer struct {
	// Meaningless while pending.                                      
	Correct                                                    *bool   `json:"correct,omitempty"`
	// Set for a multiple_choice answer.                               
	OptionID                                                   *string `json:"optionId,omitempty"`
	// True for a free_text answer not yet graded by the admin.        
	Pending                                                    bool    `json:"pending"`
	// Meaningless while pending.                                      
	PointsAwarded                                              *int64  `json:"pointsAwarded,omitempty"`
	// Set for a free_text answer.                                     
	Text                                                       *string `json:"text,omitempty"`
}

// One of a small, curated set of cosmos-themed color IDs (see the REST API's PlayerColor
// schema) — not a full color range.
type Color string

const (
	Comet   Color = "comet"
	Crimson Color = "crimson"
	Nebula  Color = "nebula"
	Nova    Color = "nova"
	Quasar  Color = "quasar"
	Solar   Color = "solar"
)

// Determines how the client should collect an answer: option buttons for multiple_choice, a
// text field (max 500 characters) for free_text. free_text questions always have an empty
// options array.
type Type string

const (
	FreeText       Type = "free_text"
	MultipleChoice Type = "multiple_choice"
)
