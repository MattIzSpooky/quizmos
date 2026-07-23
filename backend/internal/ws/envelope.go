package ws

import "encoding/json"

// Envelope is the hand-written wire format shared by every websocket
// message: {"type": "...", "payload": {...}}. Only the payload shapes are
// generated from api/asyncapi.yaml (see types.gen.go); the envelope itself
// is stable and simple enough not to warrant codegen.
type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

const (
	TypePresencePlayerJoined = "presence.playerJoined"
	TypePresencePlayerLeft   = "presence.playerLeft"
	TypeGameStarted          = "game.started"
	TypeQuestionStarted      = "question.started"
	TypeQuestionEnded        = "question.ended"
	TypeAnswerResult         = "answer.result"
	TypeLeaderboardUpdated   = "leaderboard.updated"
	TypeGameEnded            = "game.ended"
	TypeError                = "error"
	TypeAnswerSubmit         = "answer.submit"
)

func encode(msgType string, payload any) (Envelope, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{Type: msgType, Payload: raw}, nil
}
