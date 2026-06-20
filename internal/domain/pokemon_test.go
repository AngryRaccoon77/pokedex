package domain

import (
	"errors"
	"testing"
)

func TestCreatePokemonInput_Validate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreatePokemonInput
		wantErr error
	}{
		{
			name:    "valid input",
			input:   CreatePokemonInput{Name: "Pikachu", Description: "Mouse pokemon", Power: 55, Type: "electric"},
			wantErr: nil,
		},
		{
			name:    "empty name",
			input:   CreatePokemonInput{Name: "", Power: 55, Type: "electric"},
			wantErr: ErrNameRequired,
		},
		{
			name:    "negative power",
			input:   CreatePokemonInput{Name: "Pikachu", Power: -1, Type: "electric"},
			wantErr: ErrNegativePower,
		},
		{
			name:    "invalid type",
			input:   CreatePokemonInput{Name: "Pikachu", Power: 55, Type: "lightning"},
			wantErr: ErrInvalidType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpdatePokemonInput_Validate(t *testing.T) {
	emptyName := ""
	name := "Raichu"
	negPower := -5
	power := 90
	invalidType := "lightning"
	validType := "electric"
	catchable := false

	tests := []struct {
		name    string
		input   UpdatePokemonInput
		wantErr error
	}{
		{
			name:    "no fields set",
			input:   UpdatePokemonInput{},
			wantErr: nil,
		},
		{
			name:    "valid fields set",
			input:   UpdatePokemonInput{Name: &name, Power: &power, Type: &validType, Catchable: &catchable},
			wantErr: nil,
		},
		{
			name:    "empty name set",
			input:   UpdatePokemonInput{Name: &emptyName},
			wantErr: ErrNameRequired,
		},
		{
			name:    "negative power set",
			input:   UpdatePokemonInput{Power: &negPower},
			wantErr: ErrNegativePower,
		},
		{
			name:    "invalid type set",
			input:   UpdatePokemonInput{Type: &invalidType},
			wantErr: ErrInvalidType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Validate() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidType(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want bool
	}{
		{name: "valid type fire", typ: "fire", want: true},
		{name: "valid type steel", typ: "steel", want: true},
		{name: "invalid type", typ: "lightning", want: false},
		{name: "empty type", typ: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidType(tt.typ); got != tt.want {
				t.Errorf("IsValidType(%q) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}
