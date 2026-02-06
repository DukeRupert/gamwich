package grocery

import "testing"

func TestCategorizeExactMatch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"milk", "Dairy"},
		{"chicken", "Meat & Seafood"},
		{"bread", "Bakery"},
		{"rice", "Pantry"},
		{"ice cream", "Frozen"},
		{"coffee", "Beverages"},
		{"chips", "Snacks"},
		{"paper towels", "Household"},
		{"shampoo", "Personal Care"},
		{"apple", "Produce"},
	}
	for _, tt := range tests {
		got := Categorize(tt.input)
		if got != tt.want {
			t.Errorf("Categorize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCategorizeSubstringMatch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"chicken breast", "Meat & Seafood"},
		{"boneless chicken thighs", "Meat & Seafood"},
		{"whole wheat bread", "Bakery"},
		{"frozen pizza", "Frozen"},
		{"organic baby spinach", "Produce"},
		{"sparkling water bottles", "Beverages"},
		{"canned black beans", "Pantry"},
		{"dish soap refill", "Household"},
		{"greek yogurt cups", "Dairy"},
	}
	for _, tt := range tests {
		got := Categorize(tt.input)
		if got != tt.want {
			t.Errorf("Categorize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCategorizeCaseInsensitive(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MILK", "Dairy"},
		{"Chicken", "Meat & Seafood"},
		{"Frozen Pizza", "Frozen"},
		{"PAPER TOWELS", "Household"},
	}
	for _, tt := range tests {
		got := Categorize(tt.input)
		if got != tt.want {
			t.Errorf("Categorize(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestCategorizeEmptyString(t *testing.T) {
	got := Categorize("")
	if got != "Other" {
		t.Errorf("Categorize(%q) = %q, want %q", "", got, "Other")
	}
}

func TestCategorizeWhitespace(t *testing.T) {
	got := Categorize("  milk  ")
	if got != "Dairy" {
		t.Errorf("Categorize(%q) = %q, want %q", "  milk  ", got, "Dairy")
	}
}

func TestCategorizeUnknownItem(t *testing.T) {
	tests := []string{
		"widget",
		"xyz123",
		"random thing",
	}
	for _, input := range tests {
		got := Categorize(input)
		if got != "Other" {
			t.Errorf("Categorize(%q) = %q, want %q", input, got, "Other")
		}
	}
}
