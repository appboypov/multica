// Package question owns the answer settings, value validation and display
// contract used by question services (ADR-0002). Workspace membership and
// attachment ownership are checked by the service, not by this pure package.
package question

const (
	TypeText           = "text"
	TypeYesNo          = "yes_no"
	TypeSingleChoice   = "single_choice"
	TypeMultipleChoice = "multiple_choice"
	TypeConfirmFacts   = "confirm_facts"
	TypeRanking        = "ranking"
	TypeScale          = "scale"
	TypeNumber         = "number"
	TypeDate           = "date"
	TypePerson         = "person"
	TypeFile           = "file"
	TypeApproval       = "approval"
)

type TextAnswerSettings struct {
	Length      string `json:"length"`
	Placeholder string `json:"placeholder,omitempty"`
}

type TextAnswerValue struct {
	Text          string   `json:"text"`
	AttachmentIDs []string `json:"attachment_ids,omitempty"`
}

type YesNoAnswerSettings struct {
	WhyOnNo bool `json:"why_on_no"`
}

type YesNoAnswerValue struct {
	Choice string `json:"choice"`
}

type SingleChoiceAnswerSettings struct {
	Options    []string `json:"options"`
	AllowOther bool     `json:"allow_other"`
}

// Exactly one of Option and Other is present.
type SingleChoiceAnswerValue struct {
	Option *string `json:"option,omitempty"`
	Other  *string `json:"other,omitempty"`
}

type MultipleChoiceAnswerSettings struct {
	Options []string `json:"options"`
	Min     int      `json:"min"`
	Max     int      `json:"max"`
}

type MultipleChoiceAnswerValue struct {
	Options []string `json:"options"`
}

type ConfirmFactsAnswerSettings struct {
	Facts []string `json:"facts"`
}

// Correct is required; false requires a non-empty Correction.
type FactCheck struct {
	Correct    *bool  `json:"correct"`
	Correction string `json:"correction,omitempty"`
}

type ConfirmFactsAnswerValue struct {
	Facts []FactCheck `json:"facts"`
}

type RankingAnswerSettings struct {
	Items []string `json:"items"`
}

type RankingAnswerValue struct {
	Order []string `json:"order"`
}

type ScaleAnswerSettings struct {
	Min      int    `json:"min"`
	Max      int    `json:"max"`
	MinLabel string `json:"min_label,omitempty"`
	MaxLabel string `json:"max_label,omitempty"`
}

type ScaleAnswerValue struct {
	Point int `json:"point"`
}

type NumberAnswerSettings struct {
	Unit string   `json:"unit,omitempty"`
	Min  *float64 `json:"min,omitempty"`
	Max  *float64 `json:"max,omitempty"`
}

type NumberAnswerValue struct {
	Number float64 `json:"number"`
}

type DateAnswerSettings struct {
	Kind string `json:"kind"`
}

// Date is YYYY-MM-DD. Datetime, Start and End are RFC 3339 timestamps.
// Only the fields selected by the settings' Kind may be present.
type DateAnswerValue struct {
	Date     *string `json:"date,omitempty"`
	Datetime *string `json:"datetime,omitempty"`
	Start    *string `json:"start,omitempty"`
	End      *string `json:"end,omitempty"`
}

type PersonAnswerSettings struct {
	Allow string `json:"allow"`
}

type PersonAnswerValue struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type FileAnswerSettings struct {
	Accept []string `json:"accept"`
}

type FileAnswerValue struct {
	AttachmentIDs []string `json:"attachment_ids"`
}

type ApprovalAnswerSettings struct {
	Subject string        `json:"subject"`
	Link    *ApprovalLink `json:"link,omitempty"`
}

type ApprovalLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type ApprovalAnswerValue struct {
	Decision string `json:"decision"`
}
