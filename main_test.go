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
			"fakeurl/spells/fireball?level=100&system=dnd",
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

func TestSpellResponse_UnmarshalJSON(t *testing.T) {
	type fields struct {
		Spell []Spell
	}
	type args struct {
		b []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:   "valid single spell response data",
			fields: fields{Spell: []Spell{{Name: "fireball", Description: "Does the Big Boom"}}},
			args: args{b: []byte(`{
				"name": "Fireball",
				"description": "Does the Big Boom"	
			}`)},
			wantErr: false,
		},
		{
			name:   "valid multiple spell response data",
			fields: fields{Spell: []Spell{{Name: "fireball", Description: "Does the Big Boom"}, {Name: "fireball 2", Description: "Does the other Big Boom"}}},
			args: args{b: []byte(`[
				{
					"name": "Fireball",
					"description": "Does the Big Boom"	
				},
				{
					"name": "Fireball 2",
					"description": "Does the other Big Boom"	
				}
			]
			`)},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sr := &SpellResponse{
				Spell: tt.fields.Spell,
			}
			if err := sr.UnmarshalJSON(tt.args.b); (err != nil) != tt.wantErr {
				t.Errorf("SpellResponse.UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
