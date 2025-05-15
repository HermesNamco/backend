# API Documentation

## Overview

This document provides comprehensive documentation for the backend API endpoints, including request parameters, authentication requirements, and example responses.

## Authentication

The API supports the following authentication methods:

### Basic Authentication

Used for specific endpoints, with credentials provided in the `Authorization` header.

Example:
```
Authorization: Basic base64(username:password)
```

### JWT Authentication

For secured endpoints, JWT tokens must be included in the Authorization header as a Bearer token.

Example:
```
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...
```

## Common Response Format

All API responses follow a standard format:

```json
{
  "status": "success|error",
  "message": "Description of the result",
  "data": {}, // For success responses
  "error": {  // For error responses
    "code": "ERROR_CODE",
    "details": "Detailed error message"
  }
}
```

## Status Codes

- `200 OK`: Request succeeded
- `201 Created`: Resource created successfully
- `400 Bad Request`: Invalid request parameters
- `401 Unauthorized`: Authentication failure
- `403 Forbidden`: Insufficient permissions
- `404 Not Found`: Resource not found
- `422 Unprocessable Entity`: Validation errors
- `500 Internal Server Error`: Server-side error

## Endpoints

### Health Check

#### GET /api/ping

Checks if the API is running.

**Authentication**: None

**Request**: No parameters required

**Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "message": "pong"
}
```

### User Management

#### GET /api/users

Retrieves a list of all users.

**Authentication**: None (public endpoint)

**Request**: No parameters required

**Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "data": [
    {
      "id": "1",
      "name": "John Doe",
      "email": "john.doe@example.com"
    },
    {
      "id": "2",
      "name": "Jane Smith",
      "email": "jane.smith@example.com"
    }
  ]
}
```

#### GET /api/users/:id

Retrieves a specific user by ID.

**Authentication**: None (public endpoint)

**Request Parameters**:
- `id`: User ID (path parameter)

**Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "data": {
    "id": "1",
    "name": "John Doe",
    "email": "john.doe@example.com"
  }
}
```

- Status: 404 Not Found
```json
{
  "status": "error",
  "message": "User not found"
}
```

#### POST /api/users

Creates a new user.

**Authentication**: None (public endpoint)

**Request Body**:
```json
{
  "name": "New User",
  "email": "new.user@example.com",
  "password": "securepassword"
}
```

**Response**:
- Status: 201 Created
```json
{
  "status": "success",
  "data": {
    "id": "3",
    "name": "New User",
    "email": "new.user@example.com"
  }
}
```

- Status: 400 Bad Request
```json
{
  "status": "error",
  "message": "Invalid request data",
  "error": {
    "details": "email is required"
  }
}
```

#### PUT /api/users/:id

Updates an existing user.

**Authentication**: None (public endpoint)

**Request Parameters**:
- `id`: User ID (path parameter)

**Request Body**:
```json
{
  "name": "Updated Name",
  "email": "updated.email@example.com"
}
```

**Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "data": {
    "id": "1",
    "name": "Updated Name",
    "email": "updated.email@example.com"
  }
}
```

- Status: 404 Not Found
```json
{
  "status": "error",
  "message": "User not found"
}
```

#### DELETE /api/users/:id

Deletes a user.

**Authentication**: None (public endpoint)

**Request Parameters**:
- `id`: User ID (path parameter)

**Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "message": "User deleted successfully"
}
```

- Status: 404 Not Found
```json
{
  "status": "error",
  "message": "User not found"
}
```

### Administrative Endpoints

#### GET /api/admin/stats

Retrieves system statistics.

**Authentication**: Basic Authentication
- Username: `admin`
- Password: `password`

**Request**: No parameters required

**Response**:
- Status: 200 OK
```json
{
  "status": "success",
  "data": {
    "users": 100,
    "products": 50,
    "orders": 25,
    "timestamp": "2023-05-20T15:30:45Z"
  }
}
```

### Reporting System

#### POST /api/reports/create

Creates a new report.

**Authentication**: Basic Authentication
- Username: `reporter`
- Password: `secret`

**Request Body**:
```json
{
  "title": "Monthly Sales Report",
  "content": "Detailed sales information for the month of May"
}
```

**Response**:
- Status: 201 Created
```json
{
  "status": "success",
  "data": {
    "message": "Report created successfully",
    "report": {
      "title": "Monthly Sales Report",
      "content": "Detailed sales information for the month of May"
    }
  }
}
```

- Status: 400 Bad Request
```json
{
  "status": "error",
  "message": "Invalid request data",
  "error": {
    "details": "title is required"
  }
}
```

## Error Handling

All errors follow the standard error format described in the Common Response Format section. The API aims to provide clear and actionable error messages to help developers debug issues quickly.

## Rate Limiting

The API implements rate limiting to prevent abuse. If you exceed the allowed number of requests, you will receive a 429 Too Many Requests response.

## CORS Support

The API supports Cross-Origin Resource Sharing (CORS) with the following headers:
- `Access-Control-Allow-Origin: *`
- `Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type, Authorization`

## Versioning

This documentation describes API v1. Future versions will be available under different paths (e.g., `/api/v2/`).