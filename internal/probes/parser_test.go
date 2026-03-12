package probes

import (
	"math"
	"testing"
)

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

func TestParseProbeResponse_Coherence(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin float64
		wantMax float64
		wantNil bool
	}{
		{
			name:    "coherent hedge with low confidence",
			input:   "I'm not sure about this. CONFIDENCE: 20",
			wantMin: 0.7,
			wantMax: 1.0,
		},
		{
			name:    "incoherent: hedges but reports high confidence",
			input:   "I'm not sure about this. CONFIDENCE: 95",
			wantMin: 0.0,
			wantMax: 0.4,
		},
		{
			name:    "coherent confident answer",
			input:   "The answer is X because Y and Z. CONFIDENCE: 90",
			wantMin: 0.7,
			wantMax: 1.0,
		},
		{
			name:    "no confidence reported",
			input:   "Just a plain answer with no score.",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if tt.wantNil {
				if result.CoherenceScore != nil {
					t.Errorf("CoherenceScore = %v, want nil", *result.CoherenceScore)
				}
				return
			}
			if result.CoherenceScore == nil {
				t.Fatal("CoherenceScore = nil, want non-nil")
			}
			if *result.CoherenceScore < tt.wantMin || *result.CoherenceScore > tt.wantMax {
				t.Errorf("CoherenceScore = %.3f, want [%.1f, %.1f]", *result.CoherenceScore, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestParseProbeResponse_WordCount(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"short", "I don't know. CONFIDENCE: 0", 5},
		{"empty", "", 0},
		{"multi word", "The answer is X because Y and Z. CONFIDENCE: 90", 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if result.WordCount != tt.want {
				t.Errorf("WordCount = %d, want %d", result.WordCount, tt.want)
			}
		})
	}
}

func TestParseProbeResponse_Decisiveness(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantMin float64
		wantMax float64
	}{
		{
			name:    "hedge at start is decisive",
			input:   "I'm not sure about this topic. But here is some general info about it.",
			wantMin: 0.0,
			wantMax: 0.2,
		},
		{
			name:    "hedge buried late",
			input:   "Let me explain this in detail. First, the architecture uses microservices. The database layer handles persistence. The API gateway routes requests. I'm not sure about the security aspects though. CONFIDENCE: 30",
			wantMin: 0.6,
			wantMax: 1.0,
		},
		{
			name:    "confident no hedge",
			input:   "Use a LEFT JOIN for that query. CONFIDENCE: 90",
			wantMin: 0.0,
			wantMax: 0.01,
		},
		{
			name:    "no signal no confidence",
			input:   "Here is some text without any clear signal.",
			wantMin: 0.99,
			wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseProbeResponse(tt.input)
			if result.DecisivenessPos < tt.wantMin || result.DecisivenessPos > tt.wantMax {
				t.Errorf("DecisivenessPos = %.3f, want [%.2f, %.2f]", result.DecisivenessPos, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestScoreAgentProbes_Behavioral(t *testing.T) {
	conf20 := 20.0
	conf90 := 90.0
	coh09 := 0.9
	coh02 := 0.2

	results := &AgentProbeResults{
		AgentID: "test-agent",
		Details: []ProbeDetail{
			{
				ProbeType: "boundary",
				Responses: []ResponseRecord{
					{Run: 1, Temperature: 0.7, Confidence: &conf20, HedgingScore: 0.9, CoherenceScore: &coh09, WordCount: 10, DecisivenessPos: 0.05},
					{Run: 2, Temperature: 0.7, Confidence: &conf90, HedgingScore: 0.9, CoherenceScore: &coh02, WordCount: 200, DecisivenessPos: 0.8},
				},
			},
			{
				ProbeType: "calibration",
				Responses: []ResponseRecord{
					{Run: 1, Temperature: 0.7, Confidence: &conf90, HedgingScore: 0.0, CoherenceScore: &coh09, WordCount: 50, DecisivenessPos: 0.0},
				},
			},
		},
	}

	ScoreAgentProbes(results)

	// Coherence: mean of 0.9, 0.2, 0.9 = 0.667
	wantCoherence := (0.9 + 0.2 + 0.9) / 3.0
	if math.Abs(results.CoherenceScore-wantCoherence) > 0.01 {
		t.Errorf("CoherenceScore = %.3f, want ~%.3f", results.CoherenceScore, wantCoherence)
	}

	// Decisiveness: boundary probes only → 1-0.05=0.95, 1-0.8=0.2 → mean 0.575
	wantDec := (0.95 + 0.2) / 2.0
	if math.Abs(results.DecisivenessScore-wantDec) > 0.01 {
		t.Errorf("DecisivenessScore = %.3f, want ~%.3f", results.DecisivenessScore, wantDec)
	}

	// Mean word count: (10+200+50)/3 = 86.67
	wantWC := (10.0 + 200.0 + 50.0) / 3.0
	if math.Abs(results.MeanWordCount-wantWC) > 0.1 {
		t.Errorf("MeanWordCount = %.1f, want ~%.1f", results.MeanWordCount, wantWC)
	}
}

