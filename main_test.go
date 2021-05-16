package main

import (
	"context"
	"testing"
)

func TestParseQuery(t *testing.T) {
	type testCase struct {
		input             string
		expectedRemaining string
		expectedQuery     map[string]string
	}

	testData := []testCase{
		{
			"fireball system=dnd level: 100",
			"fireball",
			map[string]string{
				"system": "dnd",
				"level":  "100",
			},
		},
	}

	for _, test := range testData {
		ctx := context.Background()

		resString, resQuery := parseQuery(ctx, test.input)

		if resString != test.expectedRemaining {
			t.Errorf("parseQuery: expected %s, got %s", test.expectedRemaining, resString)
		}
		for k, v := range resQuery {
			if val, present := test.expectedQuery[k]; !present || val != v {
				t.Errorf("parseQuery: expected map %v, got map %v", test.expectedQuery, resQuery)
			}
		}
	}
}

func TestFormatGetUrl(t *testing.T) {
	type testCase struct {
		input    string
		expected string
	}

	testData := []testCase{
		{
			"fireball system=dnd level: 100",
			"fakeurl/spell/fireball?level=100&system=dnd",
		},
	}

	apiUrl = "fakeurl"

	for _, test := range testData {
		ctx := context.Background()

		res := formatGetUrl(ctx, test.input)

		if res != test.expected {
			t.Errorf("formatGetUrl: expected %s, got %s", test.expected, res)
		}
	}
}
