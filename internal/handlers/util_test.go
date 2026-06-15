package handlers

import "testing"

func TestItoa(t *testing.T) {
	tests := []struct {
		in  int
		out string
	}{
		{0, "0"},
		{1, "1"},
		{-1, "-1"},
		{100, "100"},
	}
	for _, tt := range tests {
		if got := itoa(tt.in); got != tt.out {
			t.Errorf("itoa(%d) = %q, want %q", tt.in, got, tt.out)
		}
	}
}

func TestScanInt(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := scanInt("42")
		if err != nil || got != 42 {
			t.Fatalf("scanInt(\"42\") = %d, %v; want 42, nil", got, err)
		}
	})
	t.Run("negative", func(t *testing.T) {
		got, err := scanInt("-7")
		if err != nil || got != -7 {
			t.Fatalf("scanInt(\"-7\") = %d, %v; want -7, nil", got, err)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, err := scanInt("")
		if err == nil {
			t.Fatal("expected error for empty string")
		}
	})
	t.Run("not a number", func(t *testing.T) {
		_, err := scanInt("abc")
		if err == nil {
			t.Fatal("expected error for non-numeric string")
		}
	})
	t.Run("leading space", func(t *testing.T) {
		_, err := scanInt(" 5")
		if err == nil {
			t.Fatal("expected error for string with leading space")
		}
	})
}

func TestHumanizeLocationName(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"kanto-route-4", "Route 4"},
		{"cerulean-city", "Cerulean City"},
		{"hoenn-route-101", "Route 101"},
		{"mt-moon", "Mt Moon"},
		{"Pallet Town", "Pallet Town"},
		{"Cerulean City", "Cerulean City"},
		{"", ""},
		{"kanto-safari-zone", "Safari Zone"},
		{"pokemon-tower", "Pokemon Tower"},
	}
	for _, tt := range tests {
		if got := humanizeLocationName(tt.in); got != tt.want {
			t.Errorf("humanizeLocationName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
