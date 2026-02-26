package api

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/koriwi/yazio-cli/internal/models"
)

// ErrSessionExpired is returned when the access token is invalid and the refresh token
// exchange also fails (e.g. the refresh token has expired). Callers should redirect the
// user to the login screen.
var ErrSessionExpired = errors.New("session expired")

const (
	base         = "https://yzapi.yazio.com"
	apiLogin     = "/v15/oauth/token"
	apiConsumed  = "/v15/user/consumed-items"
	apiNutrDaily = "/v15/user/consumed-items/nutrients-daily"
	apiProducts  = "/v15/products"
	apiGoals     = "/v15/user/goals"
	apiExercises = "/v15/user/exercises"
	apiWater     = "/v15/user/water-intake"
	apiRecipes   = "/v15/recipes"

	defaultClientID     = "1_4hiybetvfksgw40o0sog4s884kwc840wwso8go4k8c04goo4c"
	defaultClientSecret = "6rok2m65xuskgkgogw40wkkk8sw0osg84s8cggsc4woos4s8o"
)

func getClientID() string {
	if v := os.Getenv("YAZIO_CLIENT_ID"); v != "" {
		return v
	}
	return defaultClientID
}

func getClientSecret() string {
	if v := os.Getenv("YAZIO_CLIENT_SECRET"); v != "" {
		return v
	}
	return defaultClientSecret
}

type Client struct {
	http         *http.Client
	token        string
	refreshToken string
	onRefresh    func(accessToken, refreshToken string)
}

func New(token string) *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}, token: token}
}

// SetRefresh configures the client to automatically refresh the access token on 401
// responses. onRefresh is called with the new tokens so the caller can persist them.
func (c *Client) SetRefresh(refreshToken string, onRefresh func(accessToken, refreshToken string)) {
	c.refreshToken = refreshToken
	c.onRefresh = onRefresh
}

// rawRequest executes the HTTP request and returns the body, status code, and any error.
// It never retries — use request() for automatic 401 handling.
func (c *Client) rawRequest(method, path string, body []byte) ([]byte, int, error) {
	var r io.Reader
	if body != nil {
		r = bytes.NewBuffer(body)
	}
	req, err := http.NewRequest(method, base+path, r)
	if err != nil {
		return nil, 0, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return data, resp.StatusCode, nil
}

// request executes the HTTP request, retrying once with a refreshed token on 401.
func (c *Client) request(method, path string, body []byte) ([]byte, error) {
	data, status, err := c.rawRequest(method, path, body)
	if err != nil {
		return nil, err
	}
	if status == 401 && c.refreshToken != "" {
		if refreshErr := c.doRefresh(); refreshErr != nil {
			return nil, fmt.Errorf("HTTP 401: token refresh failed: %w", refreshErr)
		}
		data, status, err = c.rawRequest(method, path, body)
		if err != nil {
			return nil, err
		}
	}
	if status < 200 || status >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", status, string(data))
	}
	return data, nil
}

// doRefresh exchanges c.refreshToken for a new access token and updates the client state.
func (c *Client) doRefresh() error {
	resp, err := c.RefreshAccessToken(c.refreshToken)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrSessionExpired, err)
	}
	c.token = resp.AccessToken
	if resp.RefreshToken != "" {
		c.refreshToken = resp.RefreshToken
	}
	if c.onRefresh != nil {
		c.onRefresh(c.token, c.refreshToken)
	}
	return nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// Login authenticates and returns the access token and refresh token.
func (c *Client) Login(email, password string) (tokenResponse, error) {
	body := []byte(fmt.Sprintf(
		`{"client_id":%q,"client_secret":%q,"username":%q,"password":%q,"grant_type":"password"}`,
		getClientID(), getClientSecret(), email, password,
	))
	data, status, err := c.rawRequest("POST", apiLogin, body)
	if err != nil {
		return tokenResponse{}, err
	}
	if status < 200 || status >= 300 {
		return tokenResponse{}, fmt.Errorf("HTTP %d: %s", status, string(data))
	}
	var resp tokenResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return tokenResponse{}, err
	}
	if resp.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("no access_token in response")
	}
	return resp, nil
}

// RefreshAccessToken exchanges a refresh token for a new access token and refresh token.
func (c *Client) RefreshAccessToken(refreshToken string) (tokenResponse, error) {
	body := []byte(fmt.Sprintf(
		`{"client_id":%q,"client_secret":%q,"refresh_token":%q,"grant_type":"refresh_token"}`,
		getClientID(), getClientSecret(), refreshToken,
	))
	data, status, err := c.rawRequest("POST", apiLogin, body)
	if err != nil {
		return tokenResponse{}, err
	}
	if status < 200 || status >= 300 {
		return tokenResponse{}, fmt.Errorf("HTTP %d: %s", status, string(data))
	}
	var resp tokenResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return tokenResponse{}, err
	}
	if resp.AccessToken == "" {
		return tokenResponse{}, fmt.Errorf("no access_token in response")
	}
	return resp, nil
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

// GetProfile fetches the authenticated user's profile.
func (c *Client) GetProfile() (*models.UserProfile, error) {
	data, err := c.request("GET", "/v15/user", nil)
	if err != nil {
		return nil, err
	}
	var p models.UserProfile
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SearchProducts searches for products. country should be the user's country code (e.g. "DE"),
// sex should be "male" or "female" — both are required by the YAZIO API.
func (c *Client) SearchProducts(query, country, sex string) ([]models.ProductResponse, error) {
	path := fmt.Sprintf("%s/search?query=%s&language=en&countries=%s&sex=%s",
		apiProducts,
		url.QueryEscape(query),
		url.QueryEscape(country),
		url.QueryEscape(sex),
	)
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
	type productEntry struct {
		ID              string  `json:"id"`
		ProductID       string  `json:"product_id"`
		Date            string  `json:"date"`
		Daytime         string  `json:"daytime"`
		Amount          float64 `json:"amount"`
		Serving         string  `json:"serving"`
		ServingQuantity float64 `json:"serving_quantity"`
	}
	type wrapper struct {
		Products       []productEntry `json:"products"`
		RecipePortions []any          `json:"recipe_portions"`
		SimpleProducts []any          `json:"simple_products"`
	}
	w := wrapper{
		Products: []productEntry{{
			ID:              newUUID(),
			ProductID:       req.ProductID,
			Date:            req.Date,
			Daytime:         req.Daytime,
			Amount:          req.Amount,
			Serving:         req.Serving,
			ServingQuantity: req.ServingQuantity,
		}},
		RecipePortions: []any{},
		SimpleProducts: []any{},
	}
	body, err := json.Marshal(w)
	if err != nil {
		return err
	}
	_, err = c.request("POST", apiConsumed, body)
	return err
}

func newUUID() string {
	b := make([]byte, 16)
	io.ReadFull(rand.Reader, b)
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// DeleteConsumedItem removes a consumed item by its consumed-item ID.
// The API expects a DELETE to /user/consumed-items with the ID as a JSON array body.
func (c *Client) DeleteConsumedItem(consumedID string) error {
	body, err := json.Marshal([]string{consumedID})
	if err != nil {
		return err
	}
	_, err = c.request("DELETE", apiConsumed, body)
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

// PostRaw sends a raw JSON POST and returns the response body regardless of status code.
// Useful for debugging API requests.
func (c *Client) PostRaw(path, body string) (string, int, error) {
	req, err := http.NewRequest("POST", base+path, bytes.NewBufferString(body))
	if err != nil {
		return "", 0, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Content-Type", "application/json")
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
