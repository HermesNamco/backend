# Stock API Documentation

## Overview

This document provides comprehensive documentation for the stock market data API endpoints, focusing on minute-level data for A-share stocks. The API allows retrieval of historical and latest stock data, with support for both single stock queries and batch operations.

## Data Sources

The API sources stock market data from the following providers:

- **TuShare**: A professional financial data platform focusing on Chinese markets, providing comprehensive A-share market data.
- **Tonghuashun**: A leading Chinese financial information service provider with real-time market data.

Data is typically updated every minute during market trading hours (9:30 AM - 3:00 PM China Standard Time, excluding weekends and holidays).

## Authentication

Access to stock data endpoints is controlled through the following authentication methods:

### Public Endpoints

Basic stock data retrieval endpoints are publicly accessible but subject to rate limiting.

### Protected Endpoints

Administrative endpoints require JWT authentication with specific roles.

Example of JWT authentication header:
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## Rate Limiting

To prevent abuse, the API implements rate limiting:

- Public endpoints: 5 requests per 10 seconds per IP address
- Authenticated endpoints: Higher limits based on user role

When rate limits are exceeded, the API returns a 429 Too Many Requests response:
```json
{
  "status": "error",
  "message": "Rate limit exceeded. Please try again later."
}
```

## Common Response Format

All API responses follow a standard format:

```json
{
  "status": "success|error",
  "message": "Description of the result",
  "data": {}, // For success responses
  "error": {  // For error responses
    "details": "Detailed error message"
  }
}
```

## Stock Code Format

Stock codes must follow the format: `XXXXXX.YY` where:
- `XXXXXX` is a 6-digit numeric code
- `YY` is the exchange code: `SH` (Shanghai), `SZ` (Shenzhen), or `BJ` (Beijing)

Examples: `600000.SH`, `000001.SZ`, `830799.BJ`

## Endpoints

### Get Stock Data

#### GET /api/stocks/data

Retrieves minute-level data for a specific stock within a date range.

**Authentication**: None (rate limited)

**Request Parameters**:
- `code` (required): Stock code in format XXXXXX.YY
- `start_date`: Start date in format YYYY-MM-DD (default: 7 days ago)
- `end_date`: End date in format YYYY-MM-DD (default: today)
- `format`: Response format, either "json" (default) or "csv"

**Example Request**:
```
GET /api/stocks/data?code=600000.SH&start_date=2023-05-10&end_date=2023-05-15
```

**Success Response** (JSON):
- Status: 200 OK
```json
{
  "status": "success",
  "data": {
    "code": "600000.SH",
    "start_date": "2023-05-10",
    "end_date": "2023-05-15",
    "records": [
      {
        "code": "600000.SH",
        "name": "PUDONG DEVELOPMENT BANK",
        "timestamp": "2023-05-10T09:31:00Z",
        "open": 10.25,
        "close": 10.27,
        "high": 10.28,
        "low": 10.24,
        "volume": 245678,
        "amount": 2518651.32,
        "change": 0.02,
        "percent": 0.2,
        "exchange": "SH"
      },
      // Additional records...
    ],
    "count": 120
  }
}
```

**CSV Response**:
A CSV file with headers: Code, Name, Timestamp, Open, Close, High, Low, Volume, Amount, Change, Percent, Exchange

**Error Response**:
- Status: 400 Bad Request
```json
{
  "status": "error",
  "message": "Invalid stock code format",
  "error": "Stock code must be in format XXXXXX.XX"
}
```

### Get Latest Stock Data

#### GET /api/stocks/latest/:code

Retrieves the latest available minute-level data for a specific stock.

**Authentication**: None (rate limited)

**Path Parameters**:
- `code`: Stock code in format XXXXXX.YY

**Example Request**:
```
GET /api/stocks/latest/600000.SH
```

**Success Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "data": {
    "code": "600000.SH",
    "name": "PUDONG DEVELOPMENT BANK",
    "timestamp": "2023-05-20T14:59:00Z",
    "open": 10.41,
    "close": 10.42,
    "high": 10.43,
    "low": 10.41,
    "volume": 156034,
    "amount": 1625474.28,
    "change": 0.01,
    "percent": 0.1,
    "exchange": "SH"
  },
  "source": "api"
}
```

The `source` field indicates whether the data came from the API directly (`api`) or from cache (`cache`).

**Error Response**:
- Status: 400 Bad Request
```json
{
  "status": "error",
  "message": "Invalid stock code format",
  "error": "Stock code must be in format XXXXXX.XX"
}
```

### Batch Stock Data Retrieval

#### GET /api/stocks/batch

Retrieves minute-level data for multiple stocks within a date range.

**Authentication**: None (rate limited)

**Request Parameters**:
- `codes` (required): Comma-separated list of stock codes (max 100)
- `start_date`: Start date in format YYYY-MM-DD (default: 7 days ago)
- `end_date`: End date in format YYYY-MM-DD (default: today)
- `format`: Response format, either "json" (default) or "csv"

**Example Request**:
```
GET /api/stocks/batch?codes=600000.SH,000001.SZ&start_date=2023-05-19&end_date=2023-05-20
```

**Success Response** (JSON):
- Status: 200 OK
```json
{
  "status": "success",
  "data": {
    "codes": ["600000.SH", "000001.SZ"],
    "start_date": "2023-05-19",
    "end_date": "2023-05-20",
    "records": {
      "600000.SH": [
        {
          "code": "600000.SH",
          "name": "PUDONG DEVELOPMENT BANK",
          "timestamp": "2023-05-19T09:31:00Z",
          "open": 10.30,
          "close": 10.32,
          "high": 10.33,
          "low": 10.29,
          "volume": 187456,
          "amount": 1932811.52,
          "change": 0.02,
          "percent": 0.19,
          "exchange": "SH"
        },
        // Additional records...
      ],
      "000001.SZ": [
        {
          "code": "000001.SZ",
          "name": "PING AN BANK",
          "timestamp": "2023-05-19T09:31:00Z",
          "open": 15.45,
          "close": 15.48,
          "high": 15.50,
          "low": 15.44,
          "volume": 245632,
          "amount": 3798124.16,
          "change": 0.03,
          "percent": 0.19,
          "exchange": "SZ"
        },
        // Additional records...
      ]
    },
    "counts": {
      "600000.SH": 120,
      "000001.SZ": 120
    }
  }
}
```

**CSV Response**:
A single CSV file containing data for all requested stocks with headers: Code, Name, Timestamp, Open, Close, High, Low, Volume, Amount, Change, Percent, Exchange

**Error Response**:
- Status: 400 Bad Request
```json
{
  "status": "error",
  "message": "No valid stock codes provided"
}
```

### Cache Management

#### GET /api/stocks/cache/stats

Retrieves statistics about the stock data cache.

**Authentication**: JWT with "admin" or "analyst" role

**Example Request**:
```
GET /api/stocks/cache/stats
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Success Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "data": {
    "size": 8795,
    "capacity": 10000,
    "hits": 14520,
    "misses": 1230,
    "hit_rate": 0.922,
    "evictions": 125,
    "insertions": 8920,
    "last_cleanup": "2023-05-20T14:35:12Z",
    "eviction_policy": "lru",
    "expired_items": 23,
    "items_by_code": {
      "600000.SH": 120,
      "000001.SZ": 120
      // Additional stocks...
    }
  }
}
```

#### POST /api/stocks/cache/clear

Clears the stock data cache.

**Authentication**: JWT with "admin" role

**Example Request**:
```
POST /api/stocks/cache/clear
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

**Success Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "message": "Cache cleared successfully"
}
```

## Error Codes and Meanings

| Error Code | HTTP Status | Description |
|------------|-------------|-------------|
| INVALID_STOCK_CODE | 400 | Stock code format is invalid |
| API_CONNECTION_FAILED | 500 | Connection to data provider failed |
| API_RESPONSE_INVALID | 500 | Provider response could not be parsed |
| API_RATE_LIMIT_EXCEEDED | 429 | Provider API rate limit exceeded |
| API_AUTH_FAILED | 401 | Authentication with provider failed |
| NO_DATA_FOUND | 404 | No stock data available for the specified parameters |

## Data Filtering

While the API doesn't support direct filtering parameters, you can achieve filtering by:

1. **Time Range Selection**: Use `start_date` and `end_date` parameters to narrow the data range.
2. **Client-side Filtering**: Process the returned data on the client side to filter based on price, volume, or other criteria.
3. **CSV Export**: Request data in CSV format for easier filtering in spreadsheet applications.

## Pagination

The stock data API does not implement traditional pagination. Instead, it uses time-based filtering with `start_date` and `end_date` parameters to limit the amount of data returned.

For very large date ranges, consider breaking your requests into smaller time chunks to avoid timeouts and excessive data transfer.

## Best Practices

1. **Cache Usage**: Data is automatically cached. For frequently accessed stocks, subsequent requests will be faster.

2. **Batch Requests**: Use batch requests when needing data for multiple stocks to reduce the number of API calls.

3. **Date Ranges**: Keep date ranges reasonable (1-7 days recommended for minute-level data) to avoid large responses.

4. **Rate Limiting**: Implement appropriate backoff strategies in your client to handle rate limiting responses.

5. **CSV vs. JSON**: Use CSV format for data analysis and large exports. Use JSON for direct application integration.

## Versioning

This documentation describes API v1. Future versions will be available under different paths (e.g., `/api/v2/stocks/`).