package dao

import (
	"reflect"
	"testing"
)

func TestParsePositiveIDs(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []uint
		wantErr bool
	}{
		{name: "single", input: "7", want: []uint{7}},
		{name: "trim and deduplicate", input: " 3, 1,3 ", want: []uint{3, 1}},
		{name: "empty item", input: "1,,2", wantErr: true},
		{name: "zero", input: "0", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
		{name: "not a number", input: "abc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePositiveIDs(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePositiveIDs() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parsePositiveIDs() = %v, want %v", got, tt.want)
			}
		})
	}
}
