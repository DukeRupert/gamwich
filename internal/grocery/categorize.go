package grocery

import "strings"

// Categorize returns the grocery category for the given item name.
// It performs case-insensitive matching: exact match first, then substring match.
// Falls back to "Other" if no match is found.
func Categorize(itemName string) string {
	name := strings.ToLower(strings.TrimSpace(itemName))
	if name == "" {
		return "Other"
	}

	// Phase 1: exact match
	if cat, ok := exactMatch[name]; ok {
		return cat
	}

	// Phase 2: substring match (ordered longer/more-specific first)
	for _, entry := range substringMatches {
		if strings.Contains(name, entry.keyword) {
			return entry.category
		}
	}

	return "Other"
}

var exactMatch = map[string]string{
	// Produce
	"apple":       "Produce",
	"apples":      "Produce",
	"banana":      "Produce",
	"bananas":     "Produce",
	"orange":      "Produce",
	"oranges":     "Produce",
	"lemon":       "Produce",
	"lemons":      "Produce",
	"lime":        "Produce",
	"limes":       "Produce",
	"avocado":     "Produce",
	"avocados":    "Produce",
	"tomato":      "Produce",
	"tomatoes":    "Produce",
	"potato":      "Produce",
	"potatoes":    "Produce",
	"onion":       "Produce",
	"onions":      "Produce",
	"garlic":      "Produce",
	"lettuce":     "Produce",
	"spinach":     "Produce",
	"kale":        "Produce",
	"broccoli":    "Produce",
	"carrots":     "Produce",
	"celery":      "Produce",
	"cucumber":    "Produce",
	"cucumbers":   "Produce",
	"peppers":     "Produce",
	"mushrooms":   "Produce",
	"corn":        "Produce",
	"grapes":      "Produce",
	"strawberries": "Produce",
	"blueberries": "Produce",
	"raspberries": "Produce",
	"watermelon":  "Produce",
	"pineapple":   "Produce",
	"mango":       "Produce",
	"peach":       "Produce",
	"peaches":     "Produce",
	"pear":        "Produce",
	"pears":       "Produce",
	"cilantro":    "Produce",
	"basil":       "Produce",
	"parsley":     "Produce",
	"ginger":      "Produce",
	"jalapeño":    "Produce",
	"zucchini":    "Produce",
	"asparagus":   "Produce",
	"green beans": "Produce",

	// Dairy
	"milk":          "Dairy",
	"eggs":          "Dairy",
	"butter":        "Dairy",
	"cheese":        "Dairy",
	"yogurt":        "Dairy",
	"cream cheese":  "Dairy",
	"sour cream":    "Dairy",
	"heavy cream":   "Dairy",
	"half and half": "Dairy",
	"cottage cheese": "Dairy",

	// Meat & Seafood
	"chicken":       "Meat & Seafood",
	"beef":          "Meat & Seafood",
	"pork":          "Meat & Seafood",
	"turkey":        "Meat & Seafood",
	"bacon":         "Meat & Seafood",
	"sausage":       "Meat & Seafood",
	"ham":           "Meat & Seafood",
	"steak":         "Meat & Seafood",
	"salmon":        "Meat & Seafood",
	"shrimp":        "Meat & Seafood",
	"tuna":          "Meat & Seafood",
	"fish":          "Meat & Seafood",
	"ground beef":   "Meat & Seafood",
	"ground turkey": "Meat & Seafood",
	"hot dogs":      "Meat & Seafood",
	"deli meat":     "Meat & Seafood",
	"lamb":          "Meat & Seafood",
	"crab":          "Meat & Seafood",
	"lobster":       "Meat & Seafood",
	"tilapia":       "Meat & Seafood",

	// Bakery
	"bread":    "Bakery",
	"bagels":   "Bakery",
	"tortillas": "Bakery",
	"rolls":    "Bakery",
	"buns":     "Bakery",
	"muffins":  "Bakery",
	"croissants": "Bakery",
	"pita":     "Bakery",

	// Pantry
	"rice":           "Pantry",
	"pasta":          "Pantry",
	"flour":          "Pantry",
	"sugar":          "Pantry",
	"salt":           "Pantry",
	"pepper":         "Pantry",
	"oil":            "Pantry",
	"olive oil":      "Pantry",
	"vinegar":        "Pantry",
	"soy sauce":      "Pantry",
	"ketchup":        "Pantry",
	"mustard":        "Pantry",
	"mayonnaise":     "Pantry",
	"honey":          "Pantry",
	"peanut butter":  "Pantry",
	"jelly":          "Pantry",
	"jam":            "Pantry",
	"cereal":         "Pantry",
	"oatmeal":        "Pantry",
	"canned beans":   "Pantry",
	"canned tomatoes": "Pantry",
	"soup":           "Pantry",
	"broth":          "Pantry",
	"beans":          "Pantry",
	"lentils":        "Pantry",
	"nuts":           "Pantry",
	"almonds":        "Pantry",
	"spaghetti":      "Pantry",
	"noodles":        "Pantry",
	"maple syrup":    "Pantry",
	"hot sauce":      "Pantry",
	"salsa":          "Pantry",

	// Frozen
	"ice cream":      "Frozen",
	"frozen pizza":   "Frozen",
	"frozen veggies": "Frozen",
	"frozen fruit":   "Frozen",
	"frozen waffles": "Frozen",
	"popsicles":      "Frozen",

	// Beverages
	"water":       "Beverages",
	"juice":       "Beverages",
	"coffee":      "Beverages",
	"tea":         "Beverages",
	"soda":        "Beverages",
	"beer":        "Beverages",
	"wine":        "Beverages",
	"kombucha":    "Beverages",
	"lemonade":    "Beverages",
	"sparkling water": "Beverages",

	// Snacks
	"chips":      "Snacks",
	"crackers":   "Snacks",
	"cookies":    "Snacks",
	"popcorn":    "Snacks",
	"pretzels":   "Snacks",
	"granola bars": "Snacks",
	"trail mix":  "Snacks",
	"candy":      "Snacks",
	"chocolate":  "Snacks",
	"fruit snacks": "Snacks",

	// Household
	"paper towels":   "Household",
	"toilet paper":   "Household",
	"trash bags":     "Household",
	"dish soap":      "Household",
	"laundry detergent": "Household",
	"sponges":        "Household",
	"aluminum foil":  "Household",
	"plastic wrap":   "Household",
	"zip bags":       "Household",
	"ziplock bags":   "Household",
	"light bulbs":    "Household",
	"batteries":      "Household",
	"napkins":        "Household",
	"cleaning spray": "Household",
	"bleach":         "Household",

	// Personal Care
	"shampoo":     "Personal Care",
	"conditioner": "Personal Care",
	"soap":        "Personal Care",
	"body wash":   "Personal Care",
	"toothpaste":  "Personal Care",
	"toothbrush":  "Personal Care",
	"deodorant":   "Personal Care",
	"lotion":      "Personal Care",
	"sunscreen":   "Personal Care",
	"floss":       "Personal Care",
	"razors":      "Personal Care",
	"tissues":     "Personal Care",
	"band-aids":   "Personal Care",
}

type substringEntry struct {
	keyword  string
	category string
}

// Ordered with longer/more-specific keywords first for deterministic priority.
var substringMatches = []substringEntry{
	// Meat & Seafood — longer phrases first
	{"chicken breast", "Meat & Seafood"},
	{"chicken thigh", "Meat & Seafood"},
	{"chicken wing", "Meat & Seafood"},
	{"ground beef", "Meat & Seafood"},
	{"ground turkey", "Meat & Seafood"},
	{"deli meat", "Meat & Seafood"},
	{"pork chop", "Meat & Seafood"},
	{"hot dog", "Meat & Seafood"},

	// Dairy
	{"cream cheese", "Dairy"},
	{"sour cream", "Dairy"},
	{"heavy cream", "Dairy"},
	{"cottage cheese", "Dairy"},
	{"half and half", "Dairy"},
	{"greek yogurt", "Dairy"},
	{"almond milk", "Dairy"},
	{"oat milk", "Dairy"},
	{"yogurt", "Dairy"},
	{"cheese", "Dairy"},
	{"milk", "Dairy"},
	{"butter", "Dairy"},
	{"cream", "Dairy"},
	{"egg", "Dairy"},

	// Produce
	{"salad mix", "Produce"},
	{"baby spinach", "Produce"},
	{"green onion", "Produce"},
	{"sweet potato", "Produce"},
	{"bell pepper", "Produce"},
	{"cherry tomato", "Produce"},
	{"romaine", "Produce"},
	{"arugula", "Produce"},
	{"cabbage", "Produce"},
	{"cauliflower", "Produce"},
	{"squash", "Produce"},
	{"melon", "Produce"},
	{"berry", "Produce"},
	{"berries", "Produce"},
	{"fruit", "Produce"},
	{"herb", "Produce"},
	{"lettuce", "Produce"},
	{"spinach", "Produce"},
	{"kale", "Produce"},
	{"apple", "Produce"},
	{"banana", "Produce"},
	{"tomato", "Produce"},
	{"potato", "Produce"},
	{"onion", "Produce"},
	{"pepper", "Produce"},
	{"carrot", "Produce"},
	{"celery", "Produce"},

	// Bakery
	{"sourdough", "Bakery"},
	{"whole wheat", "Bakery"},
	{"bread", "Bakery"},
	{"bagel", "Bakery"},
	{"tortilla", "Bakery"},
	{"bun", "Bakery"},
	{"roll", "Bakery"},
	{"muffin", "Bakery"},
	{"croissant", "Bakery"},

	// Pantry
	{"peanut butter", "Pantry"},
	{"olive oil", "Pantry"},
	{"coconut oil", "Pantry"},
	{"maple syrup", "Pantry"},
	{"hot sauce", "Pantry"},
	{"soy sauce", "Pantry"},
	{"pasta sauce", "Pantry"},
	{"tomato sauce", "Pantry"},
	{"canned", "Pantry"},
	{"cereal", "Pantry"},
	{"oatmeal", "Pantry"},
	{"granola", "Pantry"},
	{"rice", "Pantry"},
	{"pasta", "Pantry"},
	{"noodle", "Pantry"},
	{"flour", "Pantry"},
	{"sugar", "Pantry"},
	{"spice", "Pantry"},
	{"seasoning", "Pantry"},
	{"sauce", "Pantry"},
	{"broth", "Pantry"},
	{"stock", "Pantry"},
	{"soup", "Pantry"},
	{"bean", "Pantry"},
	{"lentil", "Pantry"},

	// Frozen
	{"frozen", "Frozen"},
	{"ice cream", "Frozen"},
	{"popsicle", "Frozen"},

	// Beverages
	{"sparkling water", "Beverages"},
	{"orange juice", "Beverages"},
	{"apple juice", "Beverages"},
	{"coffee", "Beverages"},
	{"tea", "Beverages"},
	{"juice", "Beverages"},
	{"soda", "Beverages"},
	{"water", "Beverages"},
	{"beer", "Beverages"},
	{"wine", "Beverages"},
	{"drink", "Beverages"},

	// Snacks
	{"granola bar", "Snacks"},
	{"trail mix", "Snacks"},
	{"fruit snack", "Snacks"},
	{"chip", "Snacks"},
	{"cracker", "Snacks"},
	{"cookie", "Snacks"},
	{"popcorn", "Snacks"},
	{"pretzel", "Snacks"},
	{"candy", "Snacks"},
	{"chocolate", "Snacks"},
	{"snack", "Snacks"},

	// Household
	{"paper towel", "Household"},
	{"toilet paper", "Household"},
	{"trash bag", "Household"},
	{"garbage bag", "Household"},
	{"dish soap", "Household"},
	{"laundry", "Household"},
	{"detergent", "Household"},
	{"cleaner", "Household"},
	{"cleaning", "Household"},
	{"sponge", "Household"},
	{"foil", "Household"},
	{"plastic wrap", "Household"},
	{"ziplock", "Household"},
	{"battery", "Household"},
	{"light bulb", "Household"},

	// Personal Care
	{"body wash", "Personal Care"},
	{"shampoo", "Personal Care"},
	{"conditioner", "Personal Care"},
	{"toothpaste", "Personal Care"},
	{"toothbrush", "Personal Care"},
	{"deodorant", "Personal Care"},
	{"lotion", "Personal Care"},
	{"sunscreen", "Personal Care"},
	{"razor", "Personal Care"},
	{"tissue", "Personal Care"},
	{"band-aid", "Personal Care"},
}
