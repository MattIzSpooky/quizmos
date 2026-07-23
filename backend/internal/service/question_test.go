package service

import "testing"

func TestMediaLimitBytes_Images(t *testing.T) {
	for _, ct := range []string{"image/png", "image/jpeg", "image/webp", "image/gif"} {
		mediaType, maxBytes, ok := MediaLimitBytes(ct)
		if !ok {
			t.Errorf("MediaLimitBytes(%q) not ok, want accepted", ct)
			continue
		}
		if mediaType != "image" {
			t.Errorf("MediaLimitBytes(%q) mediaType = %q, want image", ct, mediaType)
		}
		if maxBytes != MaxImageMediaBytes {
			t.Errorf("MediaLimitBytes(%q) maxBytes = %d, want %d", ct, maxBytes, MaxImageMediaBytes)
		}
	}
}

func TestMediaLimitBytes_Audio(t *testing.T) {
	for _, ct := range []string{"audio/mpeg", "audio/mp3", "audio/wav", "audio/x-wav", "audio/ogg", "audio/mp4", "audio/x-m4a", "audio/webm"} {
		mediaType, maxBytes, ok := MediaLimitBytes(ct)
		if !ok {
			t.Errorf("MediaLimitBytes(%q) not ok, want accepted", ct)
			continue
		}
		if mediaType != "audio" {
			t.Errorf("MediaLimitBytes(%q) mediaType = %q, want audio", ct, mediaType)
		}
		if maxBytes != MaxAudioMediaBytes {
			t.Errorf("MediaLimitBytes(%q) maxBytes = %d, want %d", ct, maxBytes, MaxAudioMediaBytes)
		}
	}
}

func TestMediaLimitBytes_RejectsUnknownContentType(t *testing.T) {
	for _, ct := range []string{"application/pdf", "text/plain", "", "image/svg+xml"} {
		if _, _, ok := MediaLimitBytes(ct); ok {
			t.Errorf("MediaLimitBytes(%q) ok = true, want rejected", ct)
		}
	}
}

// mediaExtensions must cover exactly the same content types as
// mediaContentTypes: UploadQuestionMedia looks up the storage key's
// extension by content type after MediaLimitBytes has already accepted
// it, so a type missing from mediaExtensions would silently store the
// object with no extension instead of failing loudly.
func TestMediaExtensions_CoversAllAcceptedContentTypes(t *testing.T) {
	for ct := range mediaContentTypes {
		ext, ok := mediaExtensions[ct]
		if !ok {
			t.Errorf("mediaExtensions missing entry for accepted content type %q", ct)
			continue
		}
		if ext == "" {
			t.Errorf("mediaExtensions[%q] is empty", ct)
		}
	}
	for ct := range mediaExtensions {
		if _, ok := mediaContentTypes[ct]; !ok {
			t.Errorf("mediaExtensions has entry for %q which mediaContentTypes doesn't accept", ct)
		}
	}
}

func TestValidateOptionsForType_MultipleChoice(t *testing.T) {
	two := []QuestionOptionInput{{Text: "a"}, {Text: "b", IsCorrect: true}}
	if err := validateOptionsForType(QuestionTypeMultipleChoice, two); err != nil {
		t.Errorf("expected 2 options to be valid, got %v", err)
	}

	one := []QuestionOptionInput{{Text: "a", IsCorrect: true}}
	if err := validateOptionsForType(QuestionTypeMultipleChoice, one); err != ErrValidation {
		t.Errorf("expected ErrValidation for 1 option, got %v", err)
	}

	if err := validateOptionsForType(QuestionTypeMultipleChoice, nil); err != ErrValidation {
		t.Errorf("expected ErrValidation for 0 options, got %v", err)
	}
}

func TestValidateOptionsForType_FreeText(t *testing.T) {
	if err := validateOptionsForType(QuestionTypeFreeText, nil); err != nil {
		t.Errorf("expected no options to be valid for free_text, got %v", err)
	}

	withOptions := []QuestionOptionInput{{Text: "a"}}
	if err := validateOptionsForType(QuestionTypeFreeText, withOptions); err != ErrValidation {
		t.Errorf("expected ErrValidation for free_text with options, got %v", err)
	}
}

func TestValidateOptionsForType_RejectsUnknownType(t *testing.T) {
	if err := validateOptionsForType("essay", nil); err != ErrValidation {
		t.Errorf("expected ErrValidation for unknown question type, got %v", err)
	}
}
