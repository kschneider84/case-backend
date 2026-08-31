package surveyresponses

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	sd "github.com/case-framework/case-backend/pkg/study/exporter/survey-definition"
	studytypes "github.com/case-framework/case-backend/pkg/study/types"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestResponseExporterAccountTracking(t *testing.T) {
	mainProfile := true
	trackingInfo := AccountTrackingInfo{
		AccountID:   "pseudonymized-account-id",
		MainProfile: &mainProfile,
	}
	response := studytypes.SurveyResponse{
		ID:          primitive.NewObjectID(),
		VersionID:   "v1",
		ArrivedAt:   100,
		SubmittedAt: 100,
		Context: map[string]string{
			accountIDColumn:   "context-account-id",
			mainProfileColumn: "false",
		},
	}
	extraContextColumns := []string{accountIDColumn, mainProfileColumn}
	parser, err := NewResponseParser(
		"survey",
		[]sd.SurveyVersionPreview{{VersionID: "v1", Published: 1}},
		false,
		nil,
		"-",
		&extraContextColumns,
	)
	if err != nil {
		t.Fatal(err)
	}
	parser.EnableAccountTracking()

	t.Run("csv", func(t *testing.T) {
		var output bytes.Buffer
		exporter, err := NewResponseExporter(parser, &output, "wide")
		if err != nil {
			t.Fatal(err)
		}
		if err := exporter.WriteResponse(&response, trackingInfo); err != nil {
			t.Fatal(err)
		}
		if err := exporter.Finish(); err != nil {
			t.Fatal(err)
		}

		rows, err := csv.NewReader(strings.NewReader(output.String())).ReadAll()
		if err != nil {
			t.Fatal(err)
		}
		if rows[0][6] != accountIDColumn || rows[0][7] != mainProfileColumn {
			t.Fatalf("tracking columns missing from CSV header: %v", rows[0])
		}
		for _, column := range []string{accountIDColumn, mainProfileColumn} {
			count := 0
			for _, header := range rows[0] {
				if header == column {
					count++
				}
			}
			if count != 1 {
				t.Fatalf("expected tracking column %q once, got header: %v", column, rows[0])
			}
		}
		if rows[1][6] != trackingInfo.AccountID || rows[1][7] != "true" {
			t.Fatalf("tracking values missing from CSV row: %v", rows[1])
		}
	})

	t.Run("json", func(t *testing.T) {
		var output bytes.Buffer
		exporter, err := NewResponseExporter(parser, &output, "json")
		if err != nil {
			t.Fatal(err)
		}
		if err := exporter.WriteResponse(&response, trackingInfo); err != nil {
			t.Fatal(err)
		}
		if err := exporter.Finish(); err != nil {
			t.Fatal(err)
		}

		var result struct {
			Responses []map[string]interface{} `json:"responses"`
		}
		if err := json.NewDecoder(strings.NewReader(output.String())).Decode(&result); err != nil {
			t.Fatal(err)
		}
		if got := result.Responses[0][accountIDColumn]; got != trackingInfo.AccountID {
			t.Fatalf("unexpected account ID: %v", got)
		}
		if got := result.Responses[0][mainProfileColumn]; got != true {
			t.Fatalf("unexpected main profile value: %v", got)
		}
	})

	t.Run("missing values are empty strings", func(t *testing.T) {
		parsed, err := parser.ParseResponse(&response, AccountTrackingInfo{})
		if err != nil {
			t.Fatal(err)
		}
		result, err := parser.ResponseToFlatObj(parsed)
		if err != nil {
			t.Fatal(err)
		}
		if got := result[accountIDColumn]; got != "" {
			t.Fatalf("expected empty account ID, got %v", got)
		}
		if got := result[mainProfileColumn]; got != "" {
			t.Fatalf("expected empty main profile, got %v", got)
		}
	})

	t.Run("disabled omits tracking fields", func(t *testing.T) {
		disabledParser, err := NewResponseParser(
			"survey",
			[]sd.SurveyVersionPreview{{VersionID: "v1", Published: 1}},
			false,
			nil,
			"-",
			&extraContextColumns,
		)
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := disabledParser.ParseResponse(&response, trackingInfo)
		if err != nil {
			t.Fatal(err)
		}
		result, err := disabledParser.ResponseToFlatObj(parsed)
		if err != nil {
			t.Fatal(err)
		}
		for _, column := range []string{accountIDColumn, mainProfileColumn} {
			if _, ok := result[column]; ok {
				t.Fatalf("tracking field %q present while tracking is disabled", column)
			}
		}
	})

}
