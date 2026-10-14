package question

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

const testPersonID = "019a0e55-4d50-7000-8000-000000000001"
const testAttachmentID = "019a0e55-4d50-7000-8000-000000000002"

func TestValidateSettings(t *testing.T) {
	cases := []struct {
		name, answerType, settings string
		wantError                  bool
	}{
		{"short text", TypeText, `{"length":"short"}`, false},
		{"long text", TypeText, `{"length":"long","placeholder":"Your answer"}`, false},
		{"invalid length", TypeText, `{"length":"medium"}`, true},
		{"missing length", TypeText, `{}`, true},
		{"yes no", TypeYesNo, `{"why_on_no":true}`, false},
		{"explicit no why", TypeYesNo, `{"why_on_no":false}`, false},
		{"missing why setting", TypeYesNo, `{}`, true},
		{"wrong why shape", TypeYesNo, `{"why_on_no":"true"}`, true},
		{"single choice", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, false},
		{"too few options", TypeSingleChoice, `{"options":["per line"],"allow_other":false}`, true},
		{"duplicate options", TypeSingleChoice, `{"options":["per line","per line"],"allow_other":false}`, true},
		{"empty label", TypeSingleChoice, `{"options":[" ","per line"],"allow_other":false}`, true},
		{"multiple choice", TypeMultipleChoice, `{"options":["A","B","C"],"min":2,"max":3}`, false},
		{"zero minimum", TypeMultipleChoice, `{"options":["A","B"],"min":0,"max":2}`, false},
		{"negative minimum", TypeMultipleChoice, `{"options":["A","B"],"min":-1,"max":2}`, true},
		{"max below min", TypeMultipleChoice, `{"options":["A","B"],"min":2,"max":1}`, true},
		{"max above options", TypeMultipleChoice, `{"options":["A","B"],"min":1,"max":3}`, true},
		{"fractional count", TypeMultipleChoice, `{"options":["A","B"],"min":1.5,"max":2}`, true},
		{"facts", TypeConfirmFacts, `{"facts":["Rounding is per line"]}`, false},
		{"no facts", TypeConfirmFacts, `{"facts":[]}`, true},
		{"duplicate facts", TypeConfirmFacts, `{"facts":["A","A"]}`, true},
		{"ranking", TypeRanking, `{"items":["A","B"]}`, false},
		{"too few ranking items", TypeRanking, `{"items":["A"]}`, true},
		{"duplicate ranking items", TypeRanking, `{"items":["A","A"]}`, true},
		{"two point scale", TypeScale, `{"min":-1,"max":0}`, false},
		{"eleven point scale", TypeScale, `{"min":0,"max":10,"min_label":"Low","max_label":"High"}`, false},
		{"one point scale", TypeScale, `{"min":1,"max":1}`, true},
		{"twelve point scale", TypeScale, `{"min":0,"max":11}`, true},
		{"reversed scale", TypeScale, `{"min":5,"max":1}`, true},
		{"fractional scale", TypeScale, `{"min":1.5,"max":5}`, true},
		{"extreme scale bounds", TypeScale, `{"min":-9223372036854775808,"max":9223372036854775807}`, true},
		{"extreme narrow scale", TypeScale, `{"min":9223372036854775806,"max":9223372036854775807}`, false},
		{"number", TypeNumber, `{"unit":"EUR","min":-1.5,"max":12.25}`, false},
		{"unbounded number", TypeNumber, `{}`, false},
		{"equal number bounds", TypeNumber, `{"min":2,"max":2}`, false},
		{"reversed number bounds", TypeNumber, `{"min":3,"max":2}`, true},
		{"non numeric bound", TypeNumber, `{"min":"1"}`, true},
		{"date", TypeDate, `{"kind":"date"}`, false},
		{"datetime", TypeDate, `{"kind":"datetime"}`, false},
		{"range", TypeDate, `{"kind":"range"}`, false},
		{"unknown date kind", TypeDate, `{"kind":"month"}`, true},
		{"members", TypePerson, `{"allow":"members"}`, false},
		{"agents", TypePerson, `{"allow":"agents"}`, false},
		{"both", TypePerson, `{"allow":"both"}`, false},
		{"unknown audience", TypePerson, `{"allow":"users"}`, true},
		{"file kinds", TypeFile, `{"accept":["application/pdf",".png","image/*"]}`, false},
		{"unfiltered file", TypeFile, `{"accept":[]}`, false},
		{"invalid file kind", TypeFile, `{"accept":["pdf"]}`, true},
		{"empty extension", TypeFile, `{"accept":["."]}`, true},
		{"path as extension", TypeFile, `{"accept":[".png/anything"]}`, true},
		{"approval", TypeApproval, `{"subject":"Invoice design"}`, false},
		{"approval link", TypeApproval, `{"subject":"Invoice design","link":{"title":"Design","url":"https://example.test/design"}}`, false},
		{"blank subject", TypeApproval, `{"subject":" "}`, true},
		{"incomplete link", TypeApproval, `{"subject":"Design","link":{"url":"https://example.test"}}`, true},
		{"unsafe link", TypeApproval, `{"subject":"Design","link":{"title":"Design","url":"javascript:alert(1)"}}`, true},
		{"unknown type", "unknown", `{}`, true},
		{"null settings", TypeText, `null`, true},
		{"null length", TypeText, `{"length":null}`, true},
		{"array settings", TypeText, `[]`, true},
		{"unknown setting", TypeText, `{"length":"short","min":1}`, true},
		{"trailing JSON", TypeNumber, `{} {}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSettings(tc.answerType, json.RawMessage(tc.settings))
			if (err != nil) != tc.wantError {
				t.Fatalf("ValidateSettings(%s, %s) = %v; want error %v", tc.answerType, tc.settings, err, tc.wantError)
			}
		})
	}
}

func TestLabelCountBoundaries(t *testing.T) {
	for _, tc := range []struct {
		answerType, field string
		min               int
	}{
		{TypeSingleChoice, "options", 2},
		{TypeMultipleChoice, "options", 2},
		{TypeRanking, "items", 2},
		{TypeConfirmFacts, "facts", 1},
	} {
		for _, count := range []int{tc.min - 1, tc.min, 20, 21} {
			t.Run(fmt.Sprintf("%s/%d", tc.answerType, count), func(t *testing.T) {
				labels := make([]string, count)
				for i := range labels {
					labels[i] = fmt.Sprintf("label %d", i)
				}
				settings := map[string]any{tc.field: labels}
				if tc.answerType == TypeSingleChoice {
					settings["allow_other"] = false
				}
				if tc.answerType == TypeMultipleChoice {
					settings["min"], settings["max"] = 0, count
				}
				raw, err := json.Marshal(settings)
				if err != nil {
					t.Fatal(err)
				}
				err = ValidateSettings(tc.answerType, raw)
				wantError := count < tc.min || count > 20
				if (err != nil) != wantError {
					t.Fatalf("count %d: %v; want error %v", count, err, wantError)
				}
			})
		}
	}
}

func TestValidateAnswer(t *testing.T) {
	cases := []struct {
		name, answerType, settings, value, note string
		wantError                               bool
	}{
		{"declined in any type", TypeNumber, `{"min":1}`, `{"declined":true}`, "", false},
		{"declined with its reason", TypePerson, `{"allow":"members"}`, `{"declined":true}`, "Not my area", false},
		{"declined false is no answer", TypeNumber, `{}`, `{"declined":false}`, "", true},
		{"declined with an answer", TypeNumber, `{}`, `{"declined":true,"number":2}`, "", true},
		{"text with attachment", TypeText, `{"length":"long"}`, `{"text":"Hello @Brian\nDetails","attachment_ids":["` + testAttachmentID + `"]}`, "", false},
		{"short text", TypeText, `{"length":"short"}`, `{"text":"Per line"}`, "", false},
		{"blank text", TypeText, `{"length":"short"}`, `{"text":" \n\t"}`, "", true},
		{"attachment only is not text", TypeText, `{"length":"long"}`, `{"attachment_ids":["` + testAttachmentID + `"]}`, "", true},
		{"invalid text attachment", TypeText, `{"length":"long"}`, `{"text":"Hello","attachment_ids":["bad"]}`, "", true},
		{"yes needs no note", TypeYesNo, `{"why_on_no":true}`, `{"choice":"yes"}`, "", false},
		{"no with why", TypeYesNo, `{"why_on_no":true}`, `{"choice":"no"}`, "Customer rounds per invoice", false},
		{"no without required why", TypeYesNo, `{"why_on_no":true}`, `{"choice":"no"}`, " \n", true},
		{"no without optional why", TypeYesNo, `{"why_on_no":false}`, `{"choice":"no"}`, "", false},
		{"unknown choice", TypeYesNo, `{"why_on_no":false}`, `{"choice":"maybe"}`, "", true},
		{"single option", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":false}`, `{"option":"per line"}`, "", false},
		{"Other with text", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, `{"other":"per invoice"}`, "", false},
		{"Other prohibited", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":false}`, `{"other":"per invoice"}`, "", true},
		{"empty Other", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, `{"other":" "}`, "", true},
		{"option and Other", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, `{"option":"per line","other":"per invoice"}`, "", true},
		{"no selection", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, `{}`, "", true},
		{"unknown option", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, `{"option":"per week"}`, "", true},
		{"multiple minimum", TypeMultipleChoice, `{"options":["A","B","C"],"min":2,"max":3}`, `{"options":["A","B"]}`, "", false},
		{"multiple maximum", TypeMultipleChoice, `{"options":["A","B","C"],"min":2,"max":3}`, `{"options":["A","B","C"]}`, "", false},
		{"below minimum", TypeMultipleChoice, `{"options":["A","B","C"],"min":2,"max":3}`, `{"options":["A"]}`, "", true},
		{"above maximum", TypeMultipleChoice, `{"options":["A","B","C"],"min":1,"max":2}`, `{"options":["A","B","C"]}`, "", true},
		{"duplicates are not picks", TypeMultipleChoice, `{"options":["A","B","C"],"min":2,"max":3}`, `{"options":["A","A"]}`, "", true},
		{"unknown multiple option", TypeMultipleChoice, `{"options":["A","B","C"],"min":1,"max":3}`, `{"options":["Other"]}`, "", true},
		{"zero picks allowed", TypeMultipleChoice, `{"options":["A","B"],"min":0,"max":2}`, `{"options":[]}`, "", false},
		{"facts with correction", TypeConfirmFacts, `{"facts":["A","B","C"]}`, `{"facts":[{"correct":true},{"correct":false,"correction":"B is per invoice"},{"correct":true}]}`, "", false},
		{"incorrect without correction", TypeConfirmFacts, `{"facts":["A","B","C"]}`, `{"facts":[{"correct":true},{"correct":false},{"correct":true}]}`, "An overall note does not correct B", true},
		{"blank correction", TypeConfirmFacts, `{"facts":["A"]}`, `{"facts":[{"correct":false,"correction":" "}]}`, "", true},
		{"missing fact", TypeConfirmFacts, `{"facts":["A","B"]}`, `{"facts":[{"correct":true}]}`, "", true},
		{"extra fact", TypeConfirmFacts, `{"facts":["A"]}`, `{"facts":[{"correct":true},{"correct":true}]}`, "", true},
		{"missing correct flag", TypeConfirmFacts, `{"facts":["A"]}`, `{"facts":[{}]}`, "", true},
		{"null correct flag", TypeConfirmFacts, `{"facts":["A"]}`, `{"facts":[{"correct":null}]}`, "", true},
		{"correct with correction", TypeConfirmFacts, `{"facts":["A"]}`, `{"facts":[{"correct":true,"correction":"different"}]}`, "", true},
		{"ranking permutation", TypeRanking, `{"items":["A","B","C"]}`, `{"order":["C","A","B"]}`, "", false},
		{"missing ranking item", TypeRanking, `{"items":["A","B","C"]}`, `{"order":["A","B"]}`, "", true},
		{"duplicate ranking item", TypeRanking, `{"items":["A","B","C"]}`, `{"order":["A","B","B"]}`, "", true},
		{"foreign ranking item", TypeRanking, `{"items":["A","B","C"]}`, `{"order":["A","B","D"]}`, "", true},
		{"scale lower bound", TypeScale, `{"min":0,"max":10}`, `{"point":0}`, "", false},
		{"scale upper bound", TypeScale, `{"min":0,"max":10}`, `{"point":10}`, "", false},
		{"scale below", TypeScale, `{"min":0,"max":10}`, `{"point":-1}`, "", true},
		{"scale above", TypeScale, `{"min":0,"max":10}`, `{"point":11}`, "", true},
		{"fractional point", TypeScale, `{"min":0,"max":10}`, `{"point":1.5}`, "", true},
		{"missing point is not zero", TypeScale, `{"min":0,"max":10}`, `{}`, "", true},
		{"number lower bound", TypeNumber, `{"min":-1.5,"max":3.25}`, `{"number":-1.5}`, "", false},
		{"number upper bound", TypeNumber, `{"min":-1.5,"max":3.25}`, `{"number":3.25}`, "", false},
		{"number below", TypeNumber, `{"min":-1.5,"max":3.25}`, `{"number":-1.6}`, "", true},
		{"number above", TypeNumber, `{"min":-1.5,"max":3.25}`, `{"number":3.26}`, "", true},
		{"unbounded negative fraction", TypeNumber, `{}`, `{"number":-23.75}`, "", false},
		{"missing number is not zero", TypeNumber, `{}`, `{}`, "", true},
		{"nonfinite number", TypeNumber, `{}`, `{"number":1e999}`, "", true},
		{"date leap day", TypeDate, `{"kind":"date"}`, `{"date":"2028-02-29"}`, "", false},
		{"impossible date", TypeDate, `{"kind":"date"}`, `{"date":"2026-02-29"}`, "", true},
		{"date rejects timestamp", TypeDate, `{"kind":"date"}`, `{"date":"2026-10-13T12:00:00Z"}`, "", true},
		{"datetime with offset", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-13T14:00:00+02:00"}`, "", false},
		{"datetime needs timezone", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-13T14:00:00"}`, "", true},
		{"datetime fractional seconds", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-13T14:00:00.123Z"}`, "", false},
		{"datetime offset hours", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-13T14:00:00+24:00"}`, "", true},
		{"datetime offset minutes", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-13T14:00:00+00:60"}`, "", true},
		{"datetime comma fraction", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-13T14:00:00,123Z"}`, "", true},
		{"datetime padded hour", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-13T4:00:00Z"}`, "", true},
		{"wrong date kind", TypeDate, `{"kind":"datetime"}`, `{"date":"2026-10-13"}`, "", true},
		{"mixed date variants", TypeDate, `{"kind":"date"}`, `{"date":"2026-10-13","datetime":"2026-10-13T12:00:00Z"}`, "", true},
		{"range compared as instants", TypeDate, `{"kind":"range"}`, `{"start":"2026-10-13T14:00:00+02:00","end":"2026-10-13T12:30:00Z"}`, "", false},
		{"equal range ends", TypeDate, `{"kind":"range"}`, `{"start":"2026-10-13T12:00:00Z","end":"2026-10-13T12:00:00Z"}`, "", false},
		{"reversed range", TypeDate, `{"kind":"range"}`, `{"start":"2026-10-13T12:00:00Z","end":"2026-10-13T13:00:00+02:00"}`, "", true},
		{"range requires end", TypeDate, `{"kind":"range"}`, `{"start":"2026-10-13T12:00:00Z"}`, "", true},
		{"range needs timestamps", TypeDate, `{"kind":"range"}`, `{"start":"2026-10-13","end":"2026-10-14"}`, "", true},
		{"member", TypePerson, `{"allow":"members"}`, `{"type":"member","id":"` + testPersonID + `"}`, "", false},
		{"agent", TypePerson, `{"allow":"agents"}`, `{"type":"agent","id":"` + testPersonID + `"}`, "", false},
		{"both permits agent", TypePerson, `{"allow":"both"}`, `{"type":"agent","id":"` + testPersonID + `"}`, "", false},
		{"both permits member", TypePerson, `{"allow":"both"}`, `{"type":"member","id":"` + testPersonID + `"}`, "", false},
		{"disallowed agent", TypePerson, `{"allow":"members"}`, `{"type":"agent","id":"` + testPersonID + `"}`, "", true},
		{"disallowed member", TypePerson, `{"allow":"agents"}`, `{"type":"member","id":"` + testPersonID + `"}`, "", true},
		{"unknown person type", TypePerson, `{"allow":"both"}`, `{"type":"user","id":"` + testPersonID + `"}`, "", true},
		{"name is not ID", TypePerson, `{"allow":"members"}`, `{"type":"member","id":"Brian"}`, "", true},
		{"file", TypeFile, `{"accept":["application/pdf"]}`, `{"attachment_ids":["` + testAttachmentID + `"]}`, "", false},
		{"no file", TypeFile, `{"accept":[]}`, `{"attachment_ids":[]}`, "", true},
		{"invalid file ID", TypeFile, `{"accept":[]}`, `{"attachment_ids":["file.pdf"]}`, "", true},
		{"approve", TypeApproval, `{"subject":"Design"}`, `{"decision":"approve"}`, "", false},
		{"decline", TypeApproval, `{"subject":"Design"}`, `{"decision":"decline"}`, "", false},
		{"request changes with note", TypeApproval, `{"subject":"Design"}`, `{"decision":"request_changes"}`, "Round per line", false},
		{"request changes without note", TypeApproval, `{"subject":"Design"}`, `{"decision":"request_changes"}`, " \n", true},
		{"unknown decision", TypeApproval, `{"subject":"Design"}`, `{"decision":"accept"}`, "", true},
		{"invalid settings also rejected", TypeScale, `{"min":0,"max":11}`, `{"point":1}`, "", true},
		{"null answer", TypeNumber, `{}`, `null`, "", true},
		{"null number", TypeNumber, `{}`, `{"number":null}`, "", true},
		{"wrong value shape", TypeNumber, `{}`, `{"number":"2"}`, "", true},
		{"unknown answer field", TypeYesNo, `{"why_on_no":false}`, `{"choice":"yes","why":"hello"}`, "", true},
		{"unknown answer type", "unknown", `{}`, `{}`, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateAnswer(tc.answerType, json.RawMessage(tc.settings), json.RawMessage(tc.value), tc.note)
			if (err != nil) != tc.wantError {
				t.Fatalf("ValidateAnswer(%s, %s) = %v; want error %v", tc.answerType, tc.value, err, tc.wantError)
			}
		})
	}
}

func TestValidateSuggested(t *testing.T) {
	cases := []struct {
		name, answerType, settings, value, namedValue string
		wantError                                     bool
	}{
		{"text", TypeText, `{"length":"short"}`, `{"text":"Per line"}`, "", false},
		{"yes", TypeYesNo, `{"why_on_no":true}`, `{"choice":"yes"}`, "", false},
		{"no optional why", TypeYesNo, `{"why_on_no":false}`, `{"choice":"no"}`, "", false},
		{"no needs missing why", TypeYesNo, `{"why_on_no":true}`, `{"choice":"no"}`, "no", true},
		{"single choice", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":false}`, `{"option":"per line"}`, "", false},
		{"outside options names suggestion", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":false}`, `{"option":"per week"}`, "per week", true},
		{"multiple choice", TypeMultipleChoice, `{"options":["A","B"],"min":1,"max":2}`, `{"options":["A","B"]}`, "", false},
		{"ranking", TypeRanking, `{"items":["A","B"]}`, `{"order":["B","A"]}`, "", false},
		{"scale", TypeScale, `{"min":1,"max":5}`, `{"point":5}`, "", false},
		{"number", TypeNumber, `{"min":0}`, `{"number":2.5}`, "", false},
		{"outside number range", TypeNumber, `{"min":0,"max":10}`, `{"number":12.25}`, "12.25", true},
		{"date", TypeDate, `{"kind":"date"}`, `{"date":"2026-10-14"}`, "", false},
		{"person", TypePerson, `{"allow":"members"}`, `{"type":"member","id":"` + testPersonID + `"}`, "", false},
		{"facts cannot be suggested", TypeConfirmFacts, `{"facts":["A"]}`, `{"facts":[{"correct":true}]}`, "true", true},
		{"files cannot be suggested", TypeFile, `{"accept":[]}`, `{"attachment_ids":["` + testAttachmentID + `"]}`, testAttachmentID, true},
		{"approval cannot be suggested", TypeApproval, `{"subject":"Design"}`, `{"decision":"approve"}`, "approve", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateSuggested(tc.answerType, json.RawMessage(tc.settings), json.RawMessage(tc.value))
			if (err != nil) != tc.wantError {
				t.Fatalf("ValidateSuggested = %v; want error %v", err, tc.wantError)
			}
			if tc.wantError && err != nil && (!strings.Contains(err.Error(), "suggested answer") || !strings.Contains(err.Error(), tc.namedValue)) {
				t.Fatalf("error must identify suggested answer %q: %v", tc.namedValue, err)
			}
		})
	}
}

func TestAcceptsFile(t *testing.T) {
	cases := []struct {
		name               string
		accept             []string
		mimeType, filename string
		want               bool
	}{
		{"unfiltered", nil, "application/octet-stream", "unknown", true},
		{"exact MIME", []string{"application/pdf"}, "application/pdf", "report.bin", true},
		{"MIME parameters", []string{"text/plain"}, "text/plain; charset=utf-8", "notes", true},
		{"MIME wildcard", []string{"image/*"}, "image/png", "image", true},
		{"wrong MIME family", []string{"image/*"}, "application/pdf", "report.pdf", false},
		{"extension case insensitive", []string{".pdf"}, "application/octet-stream", "REPORT.PDF", true},
		{"any accepted kind", []string{".png", "application/pdf"}, "application/pdf", "report.bin", true},
		{"extension not substring", []string{".pdf"}, "application/octet-stream", "report.pdf.exe", false},
		{"wrong file kind", []string{".png"}, "application/pdf", "report.pdf", false},
		{"malformed MIME still allows extension", []string{".pdf"}, "invalid", "report.pdf", true},
		{"malformed MIME not accepted", []string{"image/*"}, "invalid", "report", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := AcceptsFile(FileAnswerSettings{Accept: tc.accept}, tc.mimeType, tc.filename); got != tc.want {
				t.Fatalf("AcceptsFile(%v, %q, %q) = %v; want %v", tc.accept, tc.mimeType, tc.filename, got, tc.want)
			}
		})
	}
}

func TestDisplay(t *testing.T) {
	cases := []struct{ name, answerType, settings, value, want string }{
		{"declined in any type", TypeRanking, `{"items":["A","B"]}`, `{"declined":true}`, "Declined to answer"},
		{"text is one line", TypeText, `{"length":"long"}`, `{"text":"First\nsecond\tthird"}`, "First second third"},
		{"yes", TypeYesNo, `{"why_on_no":false}`, `{"choice":"yes"}`, "Yes"},
		{"no without its separate note", TypeYesNo, `{"why_on_no":true}`, `{"choice":"no"}`, "No"},
		{"option", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, `{"option":"per line"}`, "per line"},
		{"Other is identified", TypeSingleChoice, `{"options":["per line","per total"],"allow_other":true}`, `{"other":"per invoice"}`, "Other: per invoice"},
		{"multiple", TypeMultipleChoice, `{"options":["A","B"],"min":1,"max":2}`, `{"options":["B","A"]}`, "B, A"},
		{"facts retain their statements", TypeConfirmFacts, `{"facts":["A","B"]}`, `{"facts":[{"correct":true},{"correct":false,"correction":"Per invoice"}]}`, "A: Correct; B: Not correct - Per invoice"},
		{"ranking retains order", TypeRanking, `{"items":["A","B"]}`, `{"order":["B","A"]}`, "B → A"},
		{"scale", TypeScale, `{"min":0,"max":10}`, `{"point":0}`, "0"},
		{"number with unit", TypeNumber, `{"unit":"EUR"}`, `{"number":12.25}`, "12.25 EUR"},
		{"number without unit", TypeNumber, `{}`, `{"number":-2.5}`, "-2.5"},
		{"date", TypeDate, `{"kind":"date"}`, `{"date":"2026-10-14"}`, "2026-10-14"},
		{"datetime preserves offset", TypeDate, `{"kind":"datetime"}`, `{"datetime":"2026-10-14T09:00:00+02:00"}`, "2026-10-14T09:00:00+02:00"},
		{"range", TypeDate, `{"kind":"range"}`, `{"start":"2026-10-14T09:00:00Z","end":"2026-10-14T10:00:00Z"}`, "2026-10-14T09:00:00Z – 2026-10-14T10:00:00Z"},
		{"person is an immutable reference", TypePerson, `{"allow":"members"}`, `{"type":"member","id":"` + testPersonID + `"}`, "member:" + testPersonID},
		{"file keeps reference", TypeFile, `{"accept":[]}`, `{"attachment_ids":["` + testAttachmentID + `"]}`, "1 file(s): " + testAttachmentID},
		{"approve", TypeApproval, `{"subject":"Design"}`, `{"decision":"approve"}`, "Approve"},
		{"request changes keeps note separate", TypeApproval, `{"subject":"Design"}`, `{"decision":"request_changes"}`, "Request changes"},
		{"decline", TypeApproval, `{"subject":"Design"}`, `{"decision":"decline"}`, "Decline"},
		{"unknown type", "unknown", `{}`, `{}`, ""},
		{"malformed answer", TypeNumber, `{}`, `not json`, ""},
		{"missing fact flag", TypeConfirmFacts, `{"facts":["A"]}`, `{"facts":[{}]}`, ""},
		{"missing date", TypeDate, `{"kind":"date"}`, `{}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Display(tc.answerType, json.RawMessage(tc.settings), json.RawMessage(tc.value)); got != tc.want {
				t.Fatalf("Display = %q; want %q", got, tc.want)
			}
		})
	}
}
