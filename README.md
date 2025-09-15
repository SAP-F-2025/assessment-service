# Assessment Service

A comprehensive REST API service for managing educational assessments, questions, and grading built with Go, Gin, and PostgreSQL.

## Features

- **Assessment Management**: Create, update, and manage assessments with flexible settings
- **Question Types**: Support for multiple choice, true/false, essay, fill-in-blank, matching, ordering, and short answer questions
- **Question Banks**: Organize and share question collections
- **Automated Grading**: Auto-grade objective questions with manual grading for subjective ones
- **Attempt Tracking**: Monitor student attempts with time limits and proctoring features
- **Analytics**: Detailed statistics and reporting
- **Event-Driven**: Real-time notifications via Kafka
- **Caching**: Redis integration for performance optimization

## Quick Start

### Prerequisites

- Go 1.24.6+
- PostgreSQL 12+
- Redis 6+ (optional)
- Kafka (optional, for events)

### Installation

1. **Clone the repository**
   ```bash
   git clone https://github.com/SAP-F-2025/assessment-service.git
   cd assessment-service
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up environment**
   ```bash
   cp .env.example .env
   # Edit .env with your configuration
   ```

4. **Start dependencies with Docker**
   ```bash
   cd external_service
   docker-compose up -d
   ```

5. **Run the service**
   ```bash
   go run main.go
   ```

The service will start on `http://localhost:8080`

### Docker Setup

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f assessment-service
```

## Configuration

Key environment variables:

```env
# Application
PORT=8080
ENVIRONMENT=development
LOG_LEVEL=info

# Database
DATABASE_URL=postgres://user:password@localhost:5432/assessment_db?sslmode=disable

# Redis (optional)
REDIS_URL=redis://localhost:6379/0

# Authentication
JWT_SECRET=your-secret-key

# Events (optional)
EVENTS_ENABLED=true
KAFKA_BROKERS=localhost:9092
```

See `.env.example` for complete configuration options.

## API Usage

### Authentication

All endpoints require JWT authentication:

```bash
curl -H "Authorization: Bearer <token>" \
     http://localhost:8080/api/v1/assessments
```

### Create Assessment

```bash
curl -X POST http://localhost:8080/api/v1/assessments \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "title": "Math Quiz",
    "description": "Basic mathematics test",
    "duration": 60,
    "passing_score": 70,
    "max_attempts": 3
  }'
```

### Create Question

```bash
curl -X POST http://localhost:8080/api/v1/questions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{
    "type": "multiple_choice",
    "text": "What is 2 + 2?",
    "points": 10,
    "content": {
      "options": [
        {"id": "a", "text": "3"},
        {"id": "b", "text": "4"},
        {"id": "c", "text": "5"}
      ],
      "correct_answers": ["b"]
    }
  }'
```

### Start Assessment Attempt

```bash
curl -X POST http://localhost:8080/api/v1/attempts/start \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <token>" \
  -d '{"assessment_id": 1}'
```

## Architecture

```
├── cmd/                    # Application entrypoints
├── internal/
│   ├── config/            # Configuration management
│   ├── handlers/          # HTTP handlers (controllers)
│   ├── services/          # Business logic layer
│   ├── repositories/      # Data access layer
│   ├── models/           # Data models and DTOs
│   ├── utils/            # Utilities and helpers
│   ├── cache/            # Caching layer
│   └── events/           # Event publishing
├── pkg/                  # Shared packages
├── docs/                 # Documentation
├── examples/             # Code examples
└── external_service/     # Docker compose for dependencies
```

### Key Components

- **Handlers**: HTTP request handling and routing
- **Services**: Business logic and validation
- **Repositories**: Database operations and queries
- **Models**: Data structures and validation rules
- **Events**: Asynchronous event publishing
- **Cache**: Redis-based caching for performance

## Question Types

### Multiple Choice
```json
{
  "type": "multiple_choice",
  "content": {
    "options": [
      {"id": "a", "text": "Option A"},
      {"id": "b", "text": "Option B"}
    ],
    "correct_answers": ["b"],
    "multiple_correct": false
  }
}
```

### Essay
```json
{
  "type": "essay",
  "content": {
    "min_words": 100,
    "max_words": 500,
    "rubric_criteria": ["Content", "Grammar", "Structure"]
  }
}
```

### Fill in the Blank
```json
{
  "type": "fill_blank",
  "content": {
    "template": "The capital of {blank1} is {blank2}",
    "blanks": {
      "blank1": {"accepted_answers": ["France"]},
      "blank2": {"accepted_answers": ["Paris"]}
    }
  }
}
```

## Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test ./internal/services -v
```

## Development

### Project Structure

- Follow clean architecture principles
- Separate concerns into layers (handlers → services → repositories)
- Use dependency injection for testability
- Implement proper error handling and logging

### Adding New Features

1. Define models in `internal/models/`
2. Create repository interfaces and implementations
3. Implement business logic in services
4. Add HTTP handlers for API endpoints
5. Update routing in `internal/handlers/router.go`
6. Add tests for all layers

### Code Style

- Use `gofmt` for formatting
- Follow Go naming conventions
- Add comments for exported functions
- Use structured logging
- Handle errors explicitly

## Deployment

### Production Checklist

- [ ] Set `ENVIRONMENT=production`
- [ ] Use strong `JWT_SECRET`
- [ ] Configure proper database credentials
- [ ] Set up SSL/TLS certificates
- [ ] Configure rate limiting
- [ ] Set up monitoring and logging
- [ ] Configure backup strategy

### Environment Variables

```env
# Production settings
ENVIRONMENT=production
LOG_LEVEL=warn
DEBUG=false

# Security
JWT_SECRET=<strong-random-secret>
CORS_ALLOWED_ORIGINS=https://yourdomain.com

# Performance
REDIS_URL=redis://redis-cluster:6379
MAX_REQUEST_SIZE=10485760
REQUEST_TIMEOUT=30
```

## Monitoring

### Health Check

```bash
curl http://localhost:8080/health
```

### Metrics

The service exposes metrics on `/metrics` endpoint (if enabled):

- Request duration and count
- Database connection pool stats
- Cache hit/miss rates
- Active attempts count

### Logging

Structured JSON logging with configurable levels:

```json
{
  "level": "info",
  "time": "2024-01-15T10:00:00Z",
  "msg": "Assessment created",
  "assessment_id": 123,
  "user_id": 456
}
```

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Write tests for new features
- Follow existing code patterns
- Update documentation
- Ensure all tests pass
- Use meaningful commit messages

## API Documentation

Detailed API documentation is available at:
- [API Documentation](docs/API.md)
- Swagger UI: `http://localhost:8080/swagger/` (when enabled)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

For questions and support:
- Create an issue on GitHub
- Check existing documentation
- Review code examples in `/examples`

## Changelog

### v1.0.0
- Initial release
- Core assessment management
- Question types support
- Automated grading
- Event-driven notifications