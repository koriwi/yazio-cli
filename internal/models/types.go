package models

import "encoding/json"

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
// Nutrients use dotted keys ("energy.energy", "nutrient.carb", …) and are per gram.
type ProductResponse struct {
	ID       string
	Name     string
	Nutrients ProductNutrients
	Servings  []Serving
	BaseUnit  string
}

func (p *ProductResponse) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID        string             `json:"id"`
		Name      string             `json:"name"`
		Nutrients map[string]float64 `json:"nutrients"`
		Servings  []struct {
			Amount  float64 `json:"amount"`
			Serving string  `json:"serving"`
		} `json:"servings"`
		BaseUnit string `json:"base_unit"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	p.ID = raw.ID
	p.Name = raw.Name
	p.BaseUnit = raw.BaseUnit
	p.Nutrients = ProductNutrients{
		EnergyKcal: raw.Nutrients["energy.energy"],
		Carb:       raw.Nutrients["nutrient.carb"],
		Protein:    raw.Nutrients["nutrient.protein"],
		Fat:        raw.Nutrients["nutrient.fat"],
		Sugar:      raw.Nutrients["nutrient.sugar"],
		Saturated:  raw.Nutrients["nutrient.saturated"],
		Salt:       raw.Nutrients["nutrient.salt"],
	}
	for _, s := range raw.Servings {
		p.Servings = append(p.Servings, Serving{
			Amount:  s.Amount,
			Serving: s.Serving,
		})
	}
	return nil
}

// ProductNutrients holds per-gram nutrient values.
type ProductNutrients struct {
	EnergyKcal float64 // kcal per gram
	Carb       float64 // g per gram
	Protein    float64 // g per gram
	Fat        float64 // g per gram
	Sugar      float64
	Saturated  float64
	Salt       float64
}

type Serving struct {
	Amount  float64 // grams per serving
	Serving string  // serving name ("cookie", "package", …)
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

// UserProfile is the response from GET /v9/user
type UserProfile struct {
	UUID      string `json:"uuid"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Country   string `json:"country"`
	Sex       string `json:"sex"`
	Language  string `json:"language"`
}

// AddConsumedRequest is the body for POST /v9/user/consumed-items
type AddConsumedRequest struct {
	ProductID       string  `json:"product_id"`
	Date            string  `json:"date"`
	Daytime         string  `json:"daytime"`
	Amount          float64 `json:"amount"`
	Serving         string  `json:"serving"`
	ServingQuantity float64 `json:"serving_quantity"`
	Type            string  `json:"type"`
}

// DiaryEntry is a resolved consumed item with product name and nutrients
type DiaryEntry struct {
	ConsumedID      string
	ProductID       string
	Name            string
	MealTime        string // populated from Daytime
	Amount          float64
	Serving         string
	ServingQuantity float64
	Kcal            float64
	Protein         float64
	Carbs           float64
	Fat             float64
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
