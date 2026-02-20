package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/koriwi/yazio-cli/internal/models"
)

const (
	base         = "https://yzapi.yazio.com"
	apiLogin     = "/v9/oauth/token"
	apiConsumed  = "/v9/user/consumed-items"
	apiNutrDaily = "/v9/user/consumed-items/nutrients-daily"
	apiProducts  = "/v9/products"
	apiGoals     = "/v9/user/goals"
	apiExercises = "/v9/user/exercises"
	apiWater     = "/v9/user/water-intake"
	apiRecipes   = "/v9/recipes"

	clientID     = "1_4hiybetvfksgw40o0sog4s884kwc840wwso8go4k8c04goo4c"
	clientSecret = "6rok2m65xuskgkgogw40wkkk8sw0osg84s8cggsc4woos4s8o"
)

type Client struct {
	http  *http.Client
	token string
}

func New(token string) *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}, token: token}
}

func (c *Client) request(method, path string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest(method, base+path, body)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

// Login authenticates and returns the access token.
func (c *Client) Login(email, password string) (string, error) {
	body := fmt.Sprintf(
		`{"client_id":%q,"client_secret":%q,"username":%q,"password":%q,"grant_type":"password"}`,
		clientID, clientSecret, email, password,
	)
	data, err := c.request("POST", apiLogin, bytes.NewBufferString(body))
	if err != nil {
		return "", err
	}
	var resp struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", err
	}
	if resp.AccessToken == "" {
		return "", fmt.Errorf("no access_token in response")
	}
	return resp.AccessToken, nil
}

// GetConsumedItems fetches all consumed items for a date.
func (c *Client) GetConsumedItems(date time.Time) (*models.ConsumedItemsResponse, error) {
	path := fmt.Sprintf("%s?date=%s", apiConsumed, date.Format(time.DateOnly))
	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp models.ConsumedItemsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetDailyNutrients fetches summed nutrients for a single date.
func (c *Client) GetDailyNutrients(date time.Time) (*models.DailyNutrient, error) {
	d := date.Format(time.DateOnly)
	path := fmt.Sprintf("%s?start=%s&end=%s", apiNutrDaily, d, d)
	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var entries []models.DailyNutrient
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return &models.DailyNutrient{Date: d}, nil
	}
	return &entries[0], nil
}

// GetGoals fetches calorie/macro goals for a date.
// The API returns dotted keys like "energy.energy", "nutrient.carb", etc.
func (c *Client) GetGoals(date time.Time) (*models.GoalsResponse, error) {
	path := fmt.Sprintf("%s?date=%s", apiGoals, date.Format(time.DateOnly))
	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	raw := map[string]float64{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return &models.GoalsResponse{
		EnergyKcal: raw["energy.energy"],
		Carb:       raw["nutrient.carb"],
		Protein:    raw["nutrient.protein"],
		Fat:        raw["nutrient.fat"],
		Water:      raw["water"],
	}, nil
}

// GetProduct fetches product details by ID.
func (c *Client) GetProduct(productID string) (*models.ProductResponse, error) {
	data, err := c.request("GET", apiProducts+"/"+productID, nil)
	if err != nil {
		return nil, err
	}
	var resp models.ProductResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	resp.ID = productID
	return &resp, nil
}

// GetRecipe fetches recipe details by ID.
func (c *Client) GetRecipe(recipeID string) (*models.ProductResponse, error) {
	data, err := c.request("GET", apiRecipes+"/"+recipeID, nil)
	if err != nil {
		return nil, err
	}
	var resp models.ProductResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	resp.ID = recipeID
	return &resp, nil
}

// SearchProducts searches for products by query string.
func (c *Client) SearchProducts(query string) ([]models.ProductResponse, error) {
	path := fmt.Sprintf("%s?search_term=%s&language=en", apiProducts, url.QueryEscape(query))
	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	// Try array first, then object with "products" key
	var arr []models.ProductResponse
	if err := json.Unmarshal(data, &arr); err == nil {
		return arr, nil
	}
	var obj models.ProductSearchResponse
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj.Products, nil
}

// AddConsumedItem posts a new consumed item to the diary.
func (c *Client) AddConsumedItem(req models.AddConsumedRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return err
	}
	_, err = c.request("POST", apiConsumed, bytes.NewBuffer(body))
	return err
}

// DeleteConsumedItem removes a consumed item by its consumed-item ID.
func (c *Client) DeleteConsumedItem(consumedID string) error {
	_, err := c.request("DELETE", apiConsumed+"/"+consumedID, nil)
	return err
}

// GetRaw fetches a raw API path and returns the response body as a string.
// Useful for debugging unknown endpoints.
func (c *Client) GetRaw(path string) (string, int, error) {
	req, err := http.NewRequest("GET", base+path, nil)
	if err != nil {
		return "", 0, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", resp.StatusCode, err
	}
	return string(data), resp.StatusCode, nil
}
