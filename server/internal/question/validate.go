package question

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

var timestampPattern = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

type answerDefinition struct {
	settings func(json.RawMessage) error
	answer   func(json.RawMessage, json.RawMessage, string) error
	display  func(json.RawMessage, json.RawMessage) string
	suggest  bool
}

// One registry owns settings, values, suggestions and display for each type.
var answerTypes = map[string]answerDefinition{
	TypeText:           definition([]string{"length"}, []string{"text"}, true, validateTextSettings, validateText, displayText),
	TypeYesNo:          definition([]string{"why_on_no"}, []string{"choice"}, true, validSettings[YesNoAnswerSettings], validateYesNo, displayYesNo),
	TypeSingleChoice:   definition([]string{"options", "allow_other"}, nil, true, validateSingleChoiceSettings, validateSingleChoice, displaySingleChoice),
	TypeMultipleChoice: definition([]string{"options", "min", "max"}, []string{"options"}, true, validateMultipleChoiceSettings, validateMultipleChoice, displayMultipleChoice),
	TypeConfirmFacts:   definition([]string{"facts"}, []string{"facts"}, false, validateFactsSettings, validateFacts, displayFacts),
	TypeRanking:        definition([]string{"items"}, []string{"order"}, true, validateRankingSettings, validateRanking, displayRanking),
	TypeScale:          definition([]string{"min", "max"}, []string{"point"}, true, validateScaleSettings, validateScale, displayScale),
	TypeNumber:         definition(nil, []string{"number"}, true, validateNumberSettings, validateNumber, displayNumber),
	TypeDate:           definition([]string{"kind"}, nil, true, validateDateSettings, validateDate, displayDate),
	TypePerson:         definition([]string{"allow"}, []string{"type", "id"}, true, validatePersonSettings, validatePerson, displayPerson),
	TypeFile:           definition([]string{"accept"}, []string{"attachment_ids"}, false, validateFileSettings, validateFile, displayFile),
	TypeApproval:       definition([]string{"subject"}, []string{"decision"}, false, validateApprovalSettings, validateApproval, displayApproval),
}

// ValidAnswerType reports whether answerType is one of the 12 answer types.
func ValidAnswerType(answerType string) bool {
	_, ok := answerTypes[answerType]
	return ok
}

// ValidateSettings refuses settings outside the selected answer type's contract.
func ValidateSettings(answerType string, settings json.RawMessage) error {
	def, ok := answerTypes[answerType]
	if !ok {
		return fmt.Errorf("unknown answer type %q", answerType)
	}
	return def.settings(settings)
}

// ValidateAnswer validates the shape, settings and note rules. The service must
// additionally resolve person membership and attachment ownership/acceptance.
func ValidateAnswer(answerType string, settings, value json.RawMessage, note string) error {
	def, ok := answerTypes[answerType]
	if !ok {
		return fmt.Errorf("unknown answer type %q", answerType)
	}
	return def.answer(settings, value, note)
}

// ValidateSuggested requires a value that can be sent as an answer without a
// note, and names that value in every refusal. Omission is handled by the caller.
func ValidateSuggested(answerType string, settings, value json.RawMessage) error {
	def, ok := answerTypes[answerType]
	if !ok {
		return fmt.Errorf("suggested answer %s: unknown answer type %q", value, answerType)
	}
	if !def.suggest {
		return fmt.Errorf("suggested answer %s is not allowed for %s", value, answerType)
	}
	if err := def.answer(settings, value, ""); err != nil {
		return fmt.Errorf("suggested answer %s: %w", value, err)
	}
	return nil
}

func definition[S, V any](settingsFields, valueFields []string, suggest bool, settingsValidator func(S) error, answerValidator func(S, V, string) error, display func(S, V) string) answerDefinition {
	parseSettings := func(raw json.RawMessage) (S, error) {
		settings, err := decodeObject[S](raw, "settings", settingsFields)
		if err == nil {
			err = settingsValidator(settings)
		}
		return settings, err
	}
	return answerDefinition{
		settings: func(raw json.RawMessage) error {
			_, err := parseSettings(raw)
			return err
		},
		answer: func(settingsRaw, valueRaw json.RawMessage, note string) error {
			settings, err := parseSettings(settingsRaw)
			if err != nil {
				return err
			}
			value, err := decodeObject[V](valueRaw, "answer", valueFields)
			if err != nil {
				return err
			}
			return answerValidator(settings, value, note)
		},
		display: func(settingsRaw, valueRaw json.RawMessage) string {
			settings, err := parseSettings(settingsRaw)
			if err != nil {
				return ""
			}
			value, err := decodeObject[V](valueRaw, "answer", valueFields)
			if err != nil {
				return ""
			}
			return oneLine(display(settings, value))
		},
		suggest: suggest,
	}
}

func decodeObject[T any](raw json.RawMessage, name string, required []string) (T, error) {
	var value T
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return value, fmt.Errorf("%s must be a JSON object", name)
	}
	for _, field := range required {
		if _, exists := fields[field]; !exists {
			return value, fmt.Errorf("%s.%s is required", name, field)
		}
	}
	for field, encoded := range fields {
		if bytes.Equal(bytes.TrimSpace(encoded), []byte("null")) {
			return value, fmt.Errorf("%s.%s cannot be null", name, field)
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("invalid %s: %w", name, err)
	}
	return value, nil
}

func validSettings[S any](S) error { return nil }

func validateLabels(labels []string, name string, min int) error {
	if len(labels) < min || len(labels) > 20 {
		return fmt.Errorf("%s must contain %d–20 labels", name, min)
	}
	for i, label := range labels {
		if strings.TrimSpace(label) == "" {
			return fmt.Errorf("%s must contain non-empty labels", name)
		}
		if slices.Contains(labels[:i], label) {
			return fmt.Errorf("%s contains duplicate label %q", name, label)
		}
	}
	return nil
}

func validateTextSettings(s TextAnswerSettings) error {
	if s.Length != "short" && s.Length != "long" {
		return errors.New("text length must be short or long")
	}
	return nil
}

func validateText(_ TextAnswerSettings, v TextAnswerValue, _ string) error {
	if strings.TrimSpace(v.Text) == "" {
		return errors.New("text answer cannot be empty")
	}
	return validateAttachmentIDs(v.AttachmentIDs)
}

func validateYesNo(s YesNoAnswerSettings, v YesNoAnswerValue, note string) error {
	if v.Choice != "yes" && v.Choice != "no" {
		return errors.New("choice must be yes or no")
	}
	if v.Choice == "no" && s.WhyOnNo && strings.TrimSpace(note) == "" {
		return errors.New("No needs a note explaining why")
	}
	return nil
}

func validateSingleChoiceSettings(s SingleChoiceAnswerSettings) error {
	return validateLabels(s.Options, "options", 2)
}

func validateSingleChoice(s SingleChoiceAnswerSettings, v SingleChoiceAnswerValue, _ string) error {
	if (v.Option == nil) == (v.Other == nil) {
		return errors.New("answer must contain exactly one option or Other")
	}
	if v.Option != nil {
		if !slices.Contains(s.Options, *v.Option) {
			return fmt.Errorf("answer %q is not one of the options", *v.Option)
		}
		return nil
	}
	if !s.AllowOther {
		return errors.New("Other is not allowed")
	}
	if strings.TrimSpace(*v.Other) == "" {
		return errors.New("Other needs non-empty text")
	}
	return nil
}

func validateMultipleChoiceSettings(s MultipleChoiceAnswerSettings) error {
	if err := validateLabels(s.Options, "options", 2); err != nil {
		return err
	}
	if s.Min < 0 || s.Max < s.Min || s.Max > len(s.Options) {
		return errors.New("option min and max must satisfy 0 <= min <= max <= number of options")
	}
	return nil
}

func validateMultipleChoice(s MultipleChoiceAnswerSettings, v MultipleChoiceAnswerValue, _ string) error {
	if len(v.Options) < s.Min {
		return fmt.Errorf("at least %d options are needed", s.Min)
	}
	if len(v.Options) > s.Max {
		return fmt.Errorf("at most %d options are allowed", s.Max)
	}
	return validateSelection(v.Options, s.Options, "option")
}

func validateSelection(selected, allowed []string, name string) error {
	for i, label := range selected {
		if !slices.Contains(allowed, label) {
			return fmt.Errorf("%s %q is not one of the options", name, label)
		}
		if slices.Contains(selected[:i], label) {
			return fmt.Errorf("%s %q must be distinct", name, label)
		}
	}
	return nil
}

func validateFactsSettings(s ConfirmFactsAnswerSettings) error {
	return validateLabels(s.Facts, "facts", 1)
}

func validateFacts(s ConfirmFactsAnswerSettings, v ConfirmFactsAnswerValue, _ string) error {
	if len(v.Facts) != len(s.Facts) {
		return errors.New("every fact needs a Correct or Not correct answer")
	}
	for i, fact := range v.Facts {
		if fact.Correct == nil {
			return fmt.Errorf("fact %d needs Correct or Not correct", i+1)
		}
		if !*fact.Correct && strings.TrimSpace(fact.Correction) == "" {
			return fmt.Errorf("Not correct fact %d needs a correction", i+1)
		}
		if *fact.Correct && fact.Correction != "" {
			return fmt.Errorf("Correct fact %d cannot have a correction", i+1)
		}
	}
	return nil
}

func validateRankingSettings(s RankingAnswerSettings) error {
	return validateLabels(s.Items, "items", 2)
}

func validateRanking(s RankingAnswerSettings, v RankingAnswerValue, _ string) error {
	if len(v.Order) != len(s.Items) {
		return errors.New("ranking must contain every item exactly once")
	}
	return validateSelection(v.Order, s.Items, "ranking item")
}

func validateScaleSettings(s ScaleAnswerSettings) error {
	// Ordered bounds make unsigned subtraction exact even across zero.
	if s.Max <= s.Min || uint(s.Max)-uint(s.Min) > 10 {
		return errors.New("scale must have 2–11 integer points")
	}
	return nil
}

func validateScale(s ScaleAnswerSettings, v ScaleAnswerValue, _ string) error {
	if v.Point < s.Min || v.Point > s.Max {
		return fmt.Errorf("point must be between %d and %d", s.Min, s.Max)
	}
	return nil
}

func validateNumberSettings(s NumberAnswerSettings) error {
	if s.Min != nil && s.Max != nil && *s.Min > *s.Max {
		return errors.New("number min must not exceed max")
	}
	return nil
}

func validateNumber(s NumberAnswerSettings, v NumberAnswerValue, _ string) error {
	if s.Min != nil && v.Number < *s.Min {
		return fmt.Errorf("number must be at least %g", *s.Min)
	}
	if s.Max != nil && v.Number > *s.Max {
		return fmt.Errorf("number must be at most %g", *s.Max)
	}
	return nil
}

func validateDateSettings(s DateAnswerSettings) error {
	if s.Kind != "date" && s.Kind != "datetime" && s.Kind != "range" {
		return errors.New("date kind must be date, datetime or range")
	}
	return nil
}

func validateDate(s DateAnswerSettings, v DateAnswerValue, _ string) error {
	switch s.Kind {
	case "date":
		if v.Date == nil || v.Datetime != nil || v.Start != nil || v.End != nil {
			return errors.New("date answer must contain only date")
		}
		if _, err := time.Parse(time.DateOnly, *v.Date); err != nil {
			return errors.New("date must be YYYY-MM-DD")
		}
	case "datetime":
		if v.Datetime == nil || v.Date != nil || v.Start != nil || v.End != nil {
			return errors.New("datetime answer must contain only datetime")
		}
		if _, err := parseTimestamp(*v.Datetime); err != nil {
			return errors.New("datetime must be RFC 3339")
		}
	case "range":
		if v.Start == nil || v.End == nil || v.Date != nil || v.Datetime != nil {
			return errors.New("range answer must contain start and end")
		}
		start, startErr := parseTimestamp(*v.Start)
		end, endErr := parseTimestamp(*v.End)
		if startErr != nil || endErr != nil {
			return errors.New("range start and end must be RFC 3339")
		}
		if end.Before(start) {
			return errors.New("range end must not be before start")
		}
	}
	return nil
}

func parseTimestamp(value string) (time.Time, error) {
	// time.Parse accepts some non-RFC3339 forms (comma fractions and invalid
	// offset hours/minutes), so check the wire grammar as well as the calendar.
	if !timestampPattern.MatchString(value) {
		return time.Time{}, errors.New("timestamp must be RFC 3339")
	}
	return time.Parse(time.RFC3339, value)
}

func validatePersonSettings(s PersonAnswerSettings) error {
	if s.Allow != "members" && s.Allow != "agents" && s.Allow != "both" {
		return errors.New("person allow must be members, agents or both")
	}
	return nil
}

func validatePerson(s PersonAnswerSettings, v PersonAnswerValue, _ string) error {
	if v.Type != "member" && v.Type != "agent" {
		return errors.New("person type must be member or agent")
	}
	if (s.Allow == "members" && v.Type != "member") || (s.Allow == "agents" && v.Type != "agent") {
		return fmt.Errorf("person must be one of the allowed %s", s.Allow)
	}
	if _, err := uuid.Parse(v.ID); err != nil {
		return errors.New("person id must be a UUID")
	}
	return nil
}

func validateFileSettings(s FileAnswerSettings) error {
	for _, accept := range s.Accept {
		if strings.HasPrefix(accept, ".") {
			if len(accept) > 1 && !strings.ContainsAny(accept, "/\\ \t\r\n") {
				continue
			}
		} else if mediaType, _, err := mime.ParseMediaType(accept); err == nil && strings.Contains(mediaType, "/") {
			continue
		}
		return fmt.Errorf("accepted file kind %q must be a MIME type or .extension", accept)
	}
	return nil
}

func validateFile(_ FileAnswerSettings, v FileAnswerValue, _ string) error {
	if len(v.AttachmentIDs) == 0 {
		return errors.New("at least one file is needed")
	}
	return validateAttachmentIDs(v.AttachmentIDs)
}

func validateAttachmentIDs(ids []string) error {
	for _, id := range ids {
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("attachment id %q must be a UUID", id)
		}
	}
	return nil
}

// AcceptsFile checks a resolved attachment against the question's filters.
// An empty accept list accepts all kinds, as an unfiltered file input does.
func AcceptsFile(settings FileAnswerSettings, mimeType, filename string) bool {
	if len(settings.Accept) == 0 {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		mediaType = ""
	}
	extension := filepath.Ext(filename)
	for _, accept := range settings.Accept {
		if strings.HasPrefix(accept, ".") {
			if strings.EqualFold(accept, extension) {
				return true
			}
			continue
		}
		acceptedType, _, err := mime.ParseMediaType(accept)
		if err != nil || mediaType == "" {
			continue
		}
		if strings.EqualFold(acceptedType, mediaType) {
			return true
		}
		if strings.HasSuffix(acceptedType, "/*") && strings.EqualFold(strings.TrimSuffix(acceptedType, "*"), strings.SplitN(mediaType, "/", 2)[0]+"/") {
			return true
		}
	}
	return false
}

func validateApprovalSettings(s ApprovalAnswerSettings) error {
	if strings.TrimSpace(s.Subject) == "" {
		return errors.New("approval subject cannot be empty")
	}
	if s.Link != nil {
		parsed, err := url.Parse(s.Link.URL)
		if strings.TrimSpace(s.Link.Title) == "" || err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			return errors.New("approval link needs a title and an http(s) URL")
		}
	}
	return nil
}

func validateApproval(_ ApprovalAnswerSettings, v ApprovalAnswerValue, note string) error {
	if v.Decision != "approve" && v.Decision != "request_changes" && v.Decision != "decline" {
		return errors.New("decision must be approve, request_changes or decline")
	}
	if v.Decision == "request_changes" && strings.TrimSpace(note) == "" {
		return errors.New("Request changes needs a note")
	}
	return nil
}
