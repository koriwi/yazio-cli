# YAZIO API Research

YAZIO has no official public API. All integrations documented here are community-developed, reverse-engineered from mobile app traffic, and may break at any time.

---

## Known Projects

### `saganos/yazio_public_api`

- **URL:** <https://github.com/saganos/yazio_public_api>
- **Stars:** 34
- **Contents:** `swagger.json` (OpenAPI spec) + 3 Node.js examples
- **Note:** CORS enforced — endpoints only work from HTTP or localhost clients, not from the public Swagger UI

### `juriadams/yazio` — TypeScript/JS client

- **URL:** <https://github.com/juriadams/yazio>
- **Stars:** 38, 8 forks
- **Stack:** TypeScript, Bun, Zod for schema validation
- **Base URL:** `https://yzapi.yazio.com/v15`
- **Created:** April 2024

### `controlado/go-yazio` — Go client

- **URL:** <https://github.com/controlado/go-yazio>
- **Docs:** <https://pkg.go.dev/github.com/controlado/go-yazio/pkg/yazio>
- **Stars:** 4
- **Latest:** v1.1.15 (August 2025), zero external dependencies

### `fliptheweb/yazio-mcp` — MCP server

- **URL:** <https://github.com/fliptheweb/yazio-mcp>
- **npm:** `yazio-mcp`
- **Description:** MCP (Model Context Protocol) server connecting Claude/Cursor AI to YAZIO data
- **Depends on:** `juriadams/yazio`

### `alroniks/yazio-dashboard` — React/Next.js dashboard

- **URL:** <https://github.com/alroniks/yazio-dashboard>
- **Stars:** 7
- **Updated:** November 2025

### `gdia/yazio-web-client` — Next.js web client (PoC)

- **URL:** <https://github.com/gdia/yazio-web-client>
- **Status:** Incomplete proof-of-concept

---

## Authentication

OAuth 2.0 Resource Owner Password Credentials flow with hardcoded client credentials extracted from the YAZIO mobile app (publicly known across all projects):

```
client_id:     1_4hiybetvfksgw40o0sog4s884kwc840wwso8go4k8c04goo4c
client_secret: 6rok2m65xuskgkgogw40wkkk8sw0osg84s8cggsc4woos4s8o
```

### Login

```
POST /oauth/token
Content-Type: application/json

{
  "client_id": "...",
  "client_secret": "...",
  "username": "<email>",
  "password": "<password>",
  "grant_type": "password"
}
```

Response:

```json
{
  "access_token": "...",
  "expires_in": 172800,
  "refresh_token": "...",
  "token_type": "bearer"
}
```

Access token expires in **48 hours** (`expires_in: 172800`). All subsequent requests use `Authorization: Bearer <access_token>`.

### Token Refresh

**Confirmed working.** POST to the same `/oauth/token` endpoint:

```json
{
  "client_id": "...",
  "client_secret": "...",
  "grant_type": "refresh_token",
  "refresh_token": "<refresh_token>"
}
```

Response shape is identical to the login response (access_token + refresh_token). The refresh token **rotates** — the server returns a new `access_token` and a new `refresh_token` on every call. Always save both. The refresh token lifetime is unknown; treat it as long-lived.

---

## API Versioning

The version prefix increments over time. Endpoint paths appear stable across versions; only the prefix changes:

- v5 — old (seen in early saganos examples)
- v9 — originally used by this project (`yazio-cli`)
- v12 — saganos swagger.json
- v15 — current (`juriadams/yazio`, as of April 2024) — **now used by `yazio-cli`**

Base URL pattern: `https://yzapi.yazio.com/v{N}/`

---

## Endpoints

### User Profile

| Method | Path                        | Description                                                     |
| ------ | --------------------------- | --------------------------------------------------------------- |
| GET    | `/user`                     | Full user profile                                               |
| GET    | `/user/settings`            | Boolean feature flags (water tracker, reminders, fasting, etc.) |
| GET    | `/user/dietary-preferences` | Dietary restriction string (nullable)                           |

User schema fields: `uuid`, `email`, `first_name`, `last_name`, `sex` (male/female/other), `country` (2-letter), `body_height`, `start_weight`, `weight_change_per_week`, `goal`, `date_of_birth`, `registration_date`, `timezone_offset`, `unit_length/mass/glucose/serving/energy`, `food_database_country`, `profile_image` (URL), `user_token`, `premium_type`, `login_type`, `activity_degree`, `stripe_customer_id`

### Food Diary

| Method | Path                                   | Description                                 |
| ------ | -------------------------------------- | ------------------------------------------- |
| GET    | `/user/consumed-items?date=YYYY-MM-DD` | All consumed items for a day                |
| POST   | `/user/consumed-items`                 | Add food entries to diary                   |
| DELETE | `/user/consumed-items`                 | Remove consumed items (IDs in request body) |

GET response shape:

```json
{
  "products": [ <ConsumedProduct>... ],
  "recipe_portions": [ ... ],
  "simple_products": [ ... ]
}
```

ConsumedProduct fields: `id` (UUID), `product_id` (UUID), `date`, `daytime` (breakfast/lunch/dinner/snack), `amount`, `serving`, `serving_quantity`, `type`

> **Note:** Search results use `product_id` (not `id`) and flat per-100g nutrient fields (`energy`, `carbohydrates`, `protein`, `fat`) instead of the `nutrients` map.

POST body:

```json
{
  "recipe_portions": [],
  "simple_products": [],
  "products": [
    {
      "id": "<uuid>",
      "product_id": "<uuid>",
      "date": "YYYY-MM-DD",
      "daytime": "breakfast|lunch|dinner|snack",
      "amount": 100.0,
      "serving": "gram|<serving_name>",
      "serving_quantity": 1.0
    }
  ]
}
```

The `id` in the POST body is a client-generated UUID v4 for the new consumed item entry.

DELETE body (JSON array of consumed-item UUIDs — the `id` field, **not** `product_id`):

```json
["<consumed-item-uuid>"]
```

> **Note:** `DELETE /user/consumed-items/<id>` (ID in path) returns HTTP 405. The correct form is DELETE to the base path with the ID(s) in the request body. Confirmed against `juriadams/yazio` v15 implementation. The `saganos/yazio_public_api` swagger.json (v5-era) has no DELETE endpoint at all — deletion may not have existed before v12/v15.

### Nutrients & Summary

| Method | Path                                                                   | Description                                           |
| ------ | ---------------------------------------------------------------------- | ----------------------------------------------------- |
| GET    | `/user/consumed-items/nutrients-daily?start=YYYY-MM-DD&end=YYYY-MM-DD` | Daily nutrient totals for a date range                |
| GET    | `/user/widgets/daily-summary?date=YYYY-MM-DD`                          | Full daily summary (meals, goals, steps, water, etc.) |
| GET    | `/user/goals?date=YYYY-MM-DD`                                          | User's goals (energy, macros, water, steps, weight)   |

Daily nutrient response fields: `date`, `energy`, `carb`, `protein`, `fat`, `energy_goal`

Goals keys (dotted notation): `energy.energy`, `nutrient.protein`, `nutrient.fat`, `nutrient.carb`, `activity.step`, `bodyvalue.weight`, `water`

### Products / Food Database

| Method | Path                                                           | Description            |
| ------ | -------------------------------------------------------------- | ---------------------- |
| GET    | `/products/<id>`                                               | Single product by UUID |
| GET    | `/products/search?query=...&sex=male&countries=DE&language=en` | Search food database   |

Product schema fields: `id` (UUID), `name`, `is_verified`, `is_private`, `is_deleted`, `has_ean`, `category`, `producer` (nullable), `nutrients` (map of dotted keys per gram), `servings` (array of `{serving, amount}`), `base_unit`, `eans`, `language`, `countries`, `updated_at`

Nutrient keys (all per gram): `energy.energy`, `nutrient.carb`, `nutrient.protein`, `nutrient.fat`, `nutrient.sugar`, `nutrient.saturated`, `nutrient.salt`, plus 24 total including minerals (calcium, iron, zinc) and vitamins (A, B-complex, D, E)

Search result fields per item: `score`, `name`, `product_id`, `serving`, `producer`, `energy`, `carbohydrates`, `protein`, `fat`, `countries`, `language`, `is_verified`

### Recipes

| Method | Path            | Description                                     |
| ------ | --------------- | ----------------------------------------------- |
| GET    | `/recipes/<id>` | Recipe details (same schema as ProductResponse) |

### Water Intake

| Method | Path                                 | Description            |
| ------ | ------------------------------------ | ---------------------- |
| GET    | `/user/water-intake?date=YYYY-MM-DD` | Water intake for a day |

Response fields: `water_intake` (number), `gateway` (nullable string), `source` (nullable string)

### Exercises

| Method | Path                              | Description            |
| ------ | --------------------------------- | ---------------------- |
| GET    | `/user/exercises?date=YYYY-MM-DD` | Exercise log for a day |

Response shape: `{ "training": [Exercise...], "custom_training": [Exercise...] }`

Exercise fields: `id` (UUID), `name`, `date`, `duration`, `distance`, `energy`, `steps`, `note` (nullable), `external_id` (nullable), `source` (nullable), `gateway` (nullable)

### Body Values

| Method | Path                                           | Description                         |
| ------ | ---------------------------------------------- | ----------------------------------- |
| GET    | `/user/bodyvalues/weight/last?date=YYYY-MM-DD` | Last weight entry on or before date |

Response fields: `id` (UUID), `date`, `value` (nullable number), `external_id` (nullable), `gateway` (nullable), `source` (nullable)

---

## Community Discussions

- **Intervals.icu forum** (<https://forum.intervals.icu/t/yazio-integration/122964>): Users requesting YAZIO integration; confirmed no official API. YAZIO officially integrates with Google Fit, Health Connect, Fitbit, and Garmin.
- **Zepp Health GitHub discussion** (<https://github.com/orgs/zepp-health/discussions/358>): Building a YAZIO app for ZeppOS smartwatch; referenced `saganos/yazio_public_api`.
- **Fitbit Community:** Integration requests on Fitbit forums — no API documented.

---

## Notes for yazio-cli

This project uses **v15** (`https://yzapi.yazio.com/v15/`), bumped from v9 after confirming v9 does not support `DELETE /user/consumed-items`. The `juriadams/yazio` TS client also uses v15. Endpoint paths are stable across versions; only the prefix changes.

The access token expires after 48 hours. `yazio-cli --refresh` exchanges the stored refresh token for a new access token without opening the TUI — suitable for a cron job. The refresh token is persisted in the config file alongside the access token; no password is stored.
