package models

// ConsumedItemsResponse is the response from GET /v9/user/consumed-items?date=...
type ConsumedItemsResponse struct {
	Products       []ConsumedProduct `json:"products"`
	RecipePortions []ConsumedRecipe  `json:"recipe_portions"`
	SimpleProducts []ConsumedProduct `json:"simple_products"`
}

type ConsumedProduct struct {
	ID              string  `json:"id"`
	ProductID       string  `json:"product_id"`
	Date            string  `json:"date"`
	Daytime         string  `json:"daytime"` // breakfast, lunch, dinner, snack
	Amount          float64 `json:"amount"`
	Serving         string  `json:"serving"`
	ServingQuantity float64 `json:"serving_quantity"`
	Type            string  `json:"type"`
}

type ConsumedRecipe struct {
	ID           string  `json:"id"`
	RecipeID     string  `json:"recipe_id"`
	Date         string  `json:"date"`
	Daytime      string  `json:"daytime"`
	PortionCount float64 `json:"portion_count"`
}

// ProductResponse is the response from GET /v9/products/{id}
type ProductResponse struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Nutrients ProductNutrients `json:"nutrients"`
	Servings  []Serving        `json:"servings"`
}

type ProductNutrients struct {
	// Field names confirmed from debug probe needed — keeping both variants
	EnergyKcal    float64 `json:"energy_kcal"`
	Energy        float64 `json:"energy"`
	Carbohydrates float64 `json:"carbohydrates"`
	Carb          float64 `json:"carb"`
	Protein       float64 `json:"protein"`
	Fat           float64 `json:"fat"`
	Fiber         float64 `json:"fiber"`
	Sugar         float64 `json:"sugar"`
}

// EnergyKcalVal returns whichever energy field is populated.
func (n ProductNutrients) EnergyKcalVal() float64 {
	if n.EnergyKcal != 0 {
		return n.EnergyKcal
	}
	return n.Energy
}

// CarbVal returns whichever carb field is populated.
func (n ProductNutrients) CarbVal() float64 {
	if n.Carbohydrates != 0 {
		return n.Carbohydrates
	}
	return n.Carb
}

type Serving struct {
	ID   string  `json:"id"`
	Size float64 `json:"size"`
	Unit string  `json:"unit"`
}

// DailyNutrient is one entry from GET /v9/user/consumed-items/nutrients-daily?start=...&end=...
// Actual field names confirmed from API: energy, carb, protein, fat
type DailyNutrient struct {
	Date       string  `json:"date"`
	Energy     float64 `json:"energy"`
	Carb       float64 `json:"carb"`
	Protein    float64 `json:"protein"`
	Fat        float64 `json:"fat"`
	EnergyGoal float64 `json:"energy_goal"`
}

// GoalsResponse holds parsed goals from GET /v9/user/goals?date=...
// The raw JSON uses dotted keys like "energy.energy", "nutrient.carb" — parsed in api/client.go
type GoalsResponse struct {
	EnergyKcal float64
	Carb       float64
	Protein    float64
	Fat        float64
	Water      float64
}

// ProductSearchResponse is the response from GET /v9/products/search?query=...
type ProductSearchResponse struct {
	Products []ProductResponse `json:"products"`
}

// AddConsumedRequest is the body for POST /v9/user/consumed-items
type AddConsumedRequest struct {
	ProductID       string  `json:"product_id"`
	Date            string  `json:"date"`
	Daytime         string  `json:"daytime"`
	Amount          float64 `json:"amount"`
	Serving         string  `json:"serving"`
	ServingQuantity float64 `json:"serving_quantity"`
}

// DiaryEntry is a resolved consumed item with product name and nutrients
type DiaryEntry struct {
	ConsumedID string
	ProductID  string
	Name       string
	MealTime   string // populated from Daytime
	Amount     float64
	Serving    string
	Kcal       float64
	Protein    float64
	Carbs      float64
	Fat        float64
}

func MealTimeLabel(mealTime string) string {
	switch mealTime {
	case "breakfast":
		return "Breakfast"
	case "lunch":
		return "Lunch"
	case "dinner":
		return "Dinner"
	case "snack":
		return "Snacks"
	default:
		return mealTime
	}
}

var MealTimes = []string{"breakfast", "lunch", "dinner", "snack"}
