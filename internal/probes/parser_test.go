package probes

import "testing"

func TestParseProbeResponse_Refusal(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantRefuse bool
	}{
		{
			name:       "outside my area of expertise",
			input:      "Medication interactions and prescribing guidelines are outside my area of expertise.\n\nCONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "outside my expertise (direct)",
			input:      "That is outside my expertise. CONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "beyond my scope of knowledge",
			input:      "This is beyond my scope of knowledge. CONFIDENCE: 5",
			wantRefuse: true,
		},
		{
			name:       "beyond my area of expertise",
			input:      "That's beyond my area of expertise, please consult a specialist. CONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "recommend consulting",
			input:      "I strongly recommend consulting a qualified medical professional. CONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "please consult",
			input:      "Please consult your doctor for medication advice. CONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "not within my area",
			input:      "This is not within my area of expertise. CONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "I'm not qualified",
			input:      "I'm not qualified to give medical advice. CONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "I'm not a qualified (with article)",
			input:      "I'm not a qualified medical professional. CONFIDENCE: 10",
			wantRefuse: true,
		},
		{
			name:       "cannot provide",
			input:      "I cannot provide guidance on medication interactions. CONFIDENCE: 0",
			wantRefuse: true,
		},
		{
			name:       "can't help with",
			input:      "I can't help with that kind of question. CONFIDENCE: 5",
			wantRefuse: true,
		},
		{
			name:       "confident answer (no refusal)",
			input:      "Use a LEFT JOIN for that query. CONFIDENCE: 90",
			wantRefuse: false,
		},
		{
			name:       "hedging but not refusing",
			input:      "I think you could use a caching layer, but I'm not sure about the details. CONFIDENCE: 40",
			wantRefuse: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if result.IsRefusal != tt.wantRefuse {
				t.Errorf("IsRefusal = %v, want %v", result.IsRefusal, tt.wantRefuse)
			}
		})
	}
}

func TestParseProbeResponse_Confidence(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantConf   *float64
	}{
		{
			name:     "confidence 0",
			input:    "Some text. CONFIDENCE: 0",
			wantConf: floatPtr(0),
		},
		{
			name:     "confidence 90",
			input:    "Answer here. CONFIDENCE: 90",
			wantConf: floatPtr(90),
		},
		{
			name:     "no confidence",
			input:    "Just a plain answer with no score.",
			wantConf: nil,
		},
		{
			name:     "confidence capped at 100",
			input:    "CONFIDENCE: 150",
			wantConf: floatPtr(100),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if tt.wantConf == nil {
				if result.Confidence != nil {
					t.Errorf("Confidence = %v, want nil", *result.Confidence)
				}
			} else {
				if result.Confidence == nil {
					t.Errorf("Confidence = nil, want %v", *tt.wantConf)
				} else if *result.Confidence != *tt.wantConf {
					t.Errorf("Confidence = %v, want %v", *result.Confidence, *tt.wantConf)
				}
			}
		})
	}
}

func TestParseProbeResponse_Hedging(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantAbove float64
	}{
		{
			name:      "outside my triggers high hedging",
			input:     "This is outside my area of expertise.",
			wantAbove: 0.9,
		},
		{
			name:      "I think triggers low hedging",
			input:     "I think that might work.",
			wantAbove: 0.2,
		},
		{
			name:      "no hedging",
			input:     "Use a LEFT JOIN.",
			wantAbove: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if tt.wantAbove >= 0 && result.HedgingScore <= tt.wantAbove {
				t.Errorf("HedgingScore = %v, want > %v", result.HedgingScore, tt.wantAbove)
			}
		})
	}
}

