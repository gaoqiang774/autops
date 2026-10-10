package controller

import "testing"

func TestParseImportedCabinetCell(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantCabinet  string
		wantPosition int
	}{
		{
			name:         "split cabinet and position",
			input:        "01列01柜-8",
			wantCabinet:  "01列01柜",
			wantPosition: 8,
		},
		{
			name:         "split cabinet and position with spaces",
			input:        "01列02柜 - 13",
			wantCabinet:  "01列02柜",
			wantPosition: 13,
		},
		{
			name:         "split rack alias and position",
			input:        "01列11架-10",
			wantCabinet:  "01列11架",
			wantPosition: 10,
		},
		{
			name:         "keep cabinet without position",
			input:        "01列03柜",
			wantCabinet:  "01列03柜",
			wantPosition: 0,
		},
		{
			name:         "keep non cabinet values unchanged",
			input:        "rack-01",
			wantCabinet:  "rack-01",
			wantPosition: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCabinet, gotPosition := parseImportedCabinetCell(tt.input)
			if gotCabinet != tt.wantCabinet || gotPosition != tt.wantPosition {
				t.Fatalf(
					"parseImportedCabinetCell(%q) = (%q, %d), want (%q, %d)",
					tt.input, gotCabinet, gotPosition, tt.wantCabinet, tt.wantPosition,
				)
			}
		})
	}
}

func TestParseCabinetPositionValue(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{input: "1", want: 1},
		{input: "-10", want: 10},
		{input: " 12 ", want: 12},
		{input: "0", want: 0},
		{input: "-0", want: 0},
		{input: "abc", want: 0},
	}

	for _, tt := range tests {
		if got := parseCabinetPositionValue(tt.input); got != tt.want {
			t.Fatalf("parseCabinetPositionValue(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
