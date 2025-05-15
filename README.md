# Go Web Application with Gin Framework

This is a sample Go web application built using the Gin framework. The project demonstrates a modular structure with clean separation of concerns, following best practices for Go web development.

## Project Overview

The project is a RESTful API service built with Gin that includes:

- User management endpoints with CRUD operations
- Authentication using JWT tokens
- Role-based access control
- Email reporting functionality
- A-share stock market minute-level data collection service with caching and scheduling.
- Comprehensive testing
- Environment-based configuration

## Directory Structure

```
├── .1024                  # Clacky configuration
├── .env                   # Environment variables
├── .gitignore             # Git ignore rules
├── configs/               # Application configuration
│   └── config.go          # Configuration loading and management
├── controllers/           # HTTP request handlers
│   ├── report_controller.go # Report related controllers
│   ├── stock_controller.go  # Stock data related controllers
│   └── user_controller.go   # User related controllers
├── docs/                  # Documentation
│   ├── api_docs.md        # General API documentation
│   └── stock_api_docs.md  # Stock API specific documentation
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── main.go                # Application entry point
├── middleware/            # HTTP middleware
│   └── auth_middleware.go # Authentication middleware
├── models/                # Data models
│   ├── stock.go           # Stock data model
│   └── user.go            # User model and related functions
├── routes/                # Route definitions
│   └── router.go          # Router setup and configuration
├── service/               # Business logic services
│   ├── cache/             # Caching services
│   │   └── stock_cache.go # Stock data cache implementation
│   ├── reporter/          # Reporting services
│   │   ├── api.go         # Reporter interface
│   │   └── email_reporter.go # Email implementation
│   ├── scheduler/         # Scheduling services
│   │   └── stock_scheduler.go # Stock data collection scheduler
│   └── stock/             # Stock data services
│       ├── api.go                 # Stock data provider interface
│       ├── tonghuashun_provider.go # Tonghuashun data provider
│       └── tushare_provider.go    # TuShare data provider
├── tests/                 # Tests
│   ├── router_test.go     # Router tests
│   └── stock_service_test.go # Stock service tests
└── utils/                 # Utility functions
    └── response.go        # HTTP response helpers
```

## Setup and Installation

### Prerequisites

- Go 1.24.2 or higher
- Git

### Installation Steps

1. Clone the repository:
```bash
git clone https://github.com/your-username/your-repo.git
cd your-repo
```

2. Install Go dependencies:
```bash
go mod download
```

3. Create and configure `.env` file. Start by copying the `.env.example` (if available) or create a new one.
   Below is an example for essential configurations. For stock service, see the "Stock Service Configuration" section.
```env
# Email configuration
SMTP_SERVER=smtp.163.com
SMTP_PORT=25
SMTP_USERNAME=your_username
SMTP_PASSWORD=your_password
EMAIL_FROM=your_email@163.com
EMAIL_TO=recipient_email@example.com

# Stock API Provider Configuration (example for TuShare)
STOCK_PROVIDER_TYPE=tushare
STOCK_API_KEY=your_tushare_api_token # For TuShare, this is your token
# STOCK_API_SECRET= # Required for some providers like Tonghuashun
STOCK_API_ENDPOINT=https://api.tushare.pro # Optional, defaults to provider's main endpoint

# Stock API Request Configuration
STOCK_API_TIMEOUT_SECONDS=30
STOCK_API_RATE_LIMIT=80 # e.g., TuShare free tier limit
STOCK_API_RETRY_COUNT=3
STOCK_API_RETRY_DELAY_SECONDS=1

# Stock Data Cache Configuration
STOCK_CACHE_SIZE=10000
STOCK_CACHE_TTL_MINUTES=1440 # 24 hours
STOCK_CACHE_EVICTION_POLICY=lru
STOCK_CACHE_CLEANUP_MINUTES=10

# Stock Data Scheduler Configuration
SCHEDULER_ENABLED=true
SCHEDULER_INTERVAL_MINUTES=5
SCHEDULER_INITIAL_DELAY_SECONDS=10
SCHEDULER_WORKER_POOL_SIZE=5
SCHEDULER_STOCKS=600000.SH,000001.SZ # Comma-separated list

# Server Configuration
SERVER_PORT=8080
GIN_MODE=debug
```

4. Run the application:
```bash
go run main.go
```

The server will start on port 8080 by default.

## Running Tests

To run all tests:
```bash
go test ./...
```

To run specific tests:
```bash
go test ./tests
```

To generate test coverage report:
```bash
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Configuration Options

The application uses a flexible configuration system that loads settings from environment variables and `.env` files.

### Server Configuration
- `SERVER_PORT`: HTTP server port (default: 8080)
- `SERVER_HOST`: HTTP server host (default: 0.0.0.0)
- `SERVER_READ_TIMEOUT`: Read timeout in seconds (default: 10)
- `SERVER_WRITE_TIMEOUT`: Write timeout in seconds (default: 10)
- `GIN_MODE`: Gin mode (debug, release, test) (default: debug)

### Email Configuration
- `SMTP_SERVER`: SMTP server address
- `SMTP_PORT`: SMTP server port
- `SMTP_USERNAME`: SMTP username
- `SMTP_PASSWORD`: SMTP password
- `EMAIL_FROM`: Sender email address
- `EMAIL_TO`: Recipient email address

### Database Configuration (Example, if used)
- `DB_HOST`: Database host (default: localhost)
- `DB_PORT`: Database port (default: 5432)
- `DB_USER`: Database user (default: postgres)
- `DB_PASSWORD`: Database password
- `DB_NAME`: Database name (default: app)
- `DB_SSLMODE`: SSL mode (default: disable)

### Authentication Configuration
- `JWT_SECRET`: Secret key for JWT tokens
- `JWT_EXPIRATION_HOURS`: JWT token expiration in hours (default: 24)
- `REFRESH_SECRET`: Secret key for refresh tokens
- `REFRESH_EXPIRATION_DAYS`: Refresh token expiration in days (default: 7)

### Stock Service Configuration
- `STOCK_PROVIDER_TYPE`: The data provider to use. Supported: `tushare`, `tonghuashun`. (Default: `tushare`)
- `STOCK_API_KEY`: API key for the selected stock data provider.
- `STOCK_API_SECRET`: API secret (if required by the provider, e.g., Tonghuashun).
- `STOCK_API_ENDPOINT`: Custom API endpoint for the provider (optional, uses provider default if empty).
- `STOCK_API_TIMEOUT_SECONDS`: Timeout for API requests in seconds (default: 30).
- `STOCK_API_RATE_LIMIT`: Max API requests per minute for the provider (default: 100, adjust based on your provider's limits).
- `STOCK_API_RETRY_COUNT`: Number of retries for failed API requests (default: 3).
- `STOCK_API_RETRY_DELAY_SECONDS`: Delay between retries in seconds (default: 1).
- `STOCK_CACHE_SIZE`: Maximum number of items in the stock data cache (default: 10000).
- `STOCK_CACHE_TTL_MINUTES`: Default Time-To-Live for cache entries in minutes (default: 1440, i.e., 24 hours).
- `STOCK_CACHE_EVICTION_POLICY`: Cache eviction policy (`lru`, `fifo`, `ttl`) (default: `lru`).
- `STOCK_CACHE_CLEANUP_MINUTES`: Interval for cleaning expired cache items in minutes (default: 10).
- `SCHEDULER_ENABLED`: Enable or disable the stock data collection scheduler (default: `true`, not a standard env var in config.go, handled by scheduler setup).
- `SCHEDULER_INTERVAL_MINUTES`: How often the scheduler fetches stock data in minutes (default: `5`, handled by scheduler setup).
- `SCHEDULER_INITIAL_DELAY_SECONDS`: Delay in seconds before the scheduler's first run (default: `10`, handled by scheduler setup).
- `SCHEDULER_WORKER_POOL_SIZE`: Number of concurrent workers for fetching stock data (default: `5`, handled by scheduler setup).
- `SCHEDULER_STOCKS`: Comma-separated list of stock codes to monitor (e.g., `600000.SH,000001.SZ`, handled by scheduler setup).

## Stock Data Service

The application includes a service to collect, cache, and serve A-share stock market minute-level data.

### Features
- **Data Providers**: Supports multiple data sources for stock information.
- **Caching**: Implements an in-memory cache with TTL and eviction policies to reduce API calls and improve response times.
- **Scheduler**: Periodically fetches the latest data for configured stock codes.

### Data Providers
The service currently supports:
- **TuShare**: Requires an API token. Set `STOCK_PROVIDER_TYPE=tushare` and `STOCK_API_KEY=your_tushare_token`.
- **Tonghuashun**: Requires an API Key and Secret. Set `STOCK_PROVIDER_TYPE=tonghuashun`, `STOCK_API_KEY=your_ths_key`, and `STOCK_API_SECRET=your_ths_secret`.

Configure the desired provider and its credentials in your `.env` file.

### Usage Examples

**Get minute data for a single stock:**
```
GET /api/stocks/data?code=600000.SH&start_date=2023-10-01&end_date=2023-10-01
```

**Get the latest minute data for a stock:**
```
GET /api/stocks/latest/000001.SZ
```

**Get data for multiple stocks in batch:**
```
GET /api/stocks/batch?codes=600000.SH,000001.SZ
```

For detailed API usage, request parameters, and response formats, please refer to the [Stock API Documentation](docs/stock_api_docs.md).

## API Documentation

- For general API documentation, see [docs/api_docs.md](docs/api_docs.md).
- For stock data service specific API documentation, see [docs/stock_api_docs.md](docs/stock_api_docs.md).

## Troubleshooting

### Stock Data Service Issues

- **No data is returned for a stock:**
  - Verify the stock code format (e.g., `600000.SH`).
  - Check if the `SCHEDULER_STOCKS` environment variable includes the stock code if you rely on scheduled updates.
  - Ensure the selected `STOCK_PROVIDER_TYPE` is correctly configured with a valid `STOCK_API_KEY` (and `STOCK_API_SECRET` if applicable) in your `.env` file.
  - Check the application logs for any errors from the stock data provider or scheduler.
  - The data provider might not have data for the requested stock or period.

- **API Key or Token Errors:**
  - Ensure your API key/token is active and has permissions for the required data.
  - Double-check that the key is correctly set in the `.env` file.
  - Some providers have rate limits or usage quotas per key; ensure you haven't exceeded them.

- **Rate Limit Exceeded (429 Error):**
  - The application has its own rate limiter for API endpoints.
  - The external stock data provider also has its own rate limits. You might need to adjust `STOCK_API_RATE_LIMIT` in `.env` to match your provider's plan or wait before making more requests.

- **Scheduler not running:**
  - Ensure `SCHEDULER_ENABLED=true` (if this variable is used for explicit control).
  - Check logs for scheduler startup messages or errors.
  - Verify `SCHEDULER_INTERVAL_MINUTES` is set to a reasonable value.

### General Issues

- **Configuration not loaded:** Ensure your `.env` file is in the project root and correctly formatted.
- **"Failed to start server":** Check if the `SERVER_PORT` is already in use.

## Contributing Guidelines

### Code Style
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use `gofmt` for code formatting.
- Add comments for exported functions, types, and constants.
- Write tests for new features and bug fixes.

### Pull Request Process
1. Fork the repository.
2. Create a feature branch (`git checkout -b feature/your-feature-name`).
3. Add and commit your changes (`git commit -am 'feat: Add some feature'`).
4. Push to your fork (`git push origin feature/your-feature-name`).
5. Create a pull request against the `main` branch of the original repository.

### Commit Message Format
Follow a conventional commit message format (e.g., `feat: ...`, `fix: ...`, `docs: ...`).
```
feat: Add new feature related to X
fix: Resolve bug Y in component Z
docs: Update README with new configuration options
style: Apply gofmt and improve code readability
refactor: Restructure user authentication logic
test: Add unit tests for the stock caching service
chore: Update Go version in build scripts
```

## License

This project is licensed under the MIT License. See the `LICENSE` file for details (if one exists in the project).