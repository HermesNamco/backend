# Go Web Application with Gin Framework

This is a sample Go web application built using the Gin framework. The project demonstrates a modular structure with clean separation of concerns, following best practices for Go web development.

## Project Overview

The project is a RESTful API service built with Gin that includes:

- User management endpoints with CRUD operations
- Authentication using JWT tokens
- Role-based access control
- Email reporting functionality
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
│   └── user_controller.go # User related controllers
├── docs/                  # Documentation
│   └── api_docs.md        # API documentation
├── go.mod                 # Go module definition
├── go.sum                 # Go module checksums
├── main.go                # Application entry point
├── middleware/            # HTTP middleware
│   └── auth_middleware.go # Authentication middleware
├── models/                # Data models
│   └── user.go            # User model and related functions
├── routes/                # Route definitions
│   └── router.go          # Router setup and configuration
├── service/               # Business logic services
│   └── reporter/          # Reporting services
│       ├── api.go         # Reporter interface
│       └── email_reporter.go # Email implementation
├── tests/                 # Tests
│   └── router_test.go     # Router tests
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

3. Create and configure `.env` file with the following variables:
```
SMTP_SERVER=smtp.163.com
SMTP_PORT=25
SMTP_USERNAME=your_username
SMTP_PASSWORD=your_password
EMAIL_FROM=your_email@163.com
EMAIL_TO=recipient_email@example.com
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

The application uses a flexible configuration system that loads settings from environment variables and `.env` files. The primary configuration categories include:

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

### Database Configuration
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

## API Documentation

For detailed API documentation, see [docs/api_docs.md](docs/api_docs.md).

## Contributing Guidelines

### Code Style
- Follow the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- Use gofmt for code formatting
- Add comments for exported functions, types, and constants
- Write tests for new features and bug fixes

### Pull Request Process
1. Fork the repository
2. Create a feature branch
3. Add and commit your changes
4. Push to your fork
5. Create a pull request

### Commit Message Format
```
feat: Add new feature
fix: Fix bug
docs: Update documentation
style: Code style changes (formatting, missing semicolons, etc)
refactor: Code refactoring
test: Add missing tests
chore: Changes to the build process or auxiliary tools
```

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.