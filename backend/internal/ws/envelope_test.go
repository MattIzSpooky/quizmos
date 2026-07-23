package ws

import (
	"encoding/json"
	"testing"
)

func TestEncode_RoundTrips(t *testing.T) {
	env, err := encode(TypeAnswerResult, AnswerResult{
		QuestionID:    "q1",
		Correct:       true,
		PointsAwarded: 100,
		TotalScore:    100,
	})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if env.Type != TypeAnswerResult {
		t.Errorf("Type = %q, want %q", env.Type, TypeAnswerResult)
	}

	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	var decoded Envelope
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal envelope: %v", err)
	}
	if decoded.Type != TypeAnswerResult {
		t.Errorf("decoded.Type = %q, want %q", decoded.Type, TypeAnswerResult)
	}

	var payload AnswerResult
	if err := json.Unmarshal(decoded.Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload != (AnswerResult{QuestionID: "q1", Correct: true, PointsAwarded: 100, TotalScore: 100}) {
		t.Errorf("payload = %+v, want round-tripped original", payload)
	}
}

func TestEnvelope_WireFormatMatchesAsyncAPIContract(t *testing.T) {
	raw := []byte(`{"type":"answer.submit","payload":{"questionId":"q1","optionId":"o1"}}`)

	var env Envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if env.Type != TypeAnswerSubmit {
		t.Errorf("Type = %q, want %q", env.Type, TypeAnswerSubmit)
	}

	var submit AnswerSubmit
	if err := json.Unmarshal(env.Payload, &submit); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if submit.QuestionID != "q1" || submit.OptionID != "o1" {
		t.Errorf("submit = %+v, want {QuestionID: q1, OptionID: o1}", submit)
	}
}
