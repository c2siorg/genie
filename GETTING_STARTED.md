# Getting Started with Genie

Welcome! This guide will help you get Genie running locally in 10-15 minutes.

## Quick Start (5 minutes)

### Prerequisites
- **Go** 1.25.0+
- **Docker** & **Docker Compose** (recommended)
- **Git**

### Clone & Setup
```bash
git clone https://github.com/c2siorg/genie.git
cd genie
cp .env.example .env
docker-compose up -d
```

### Verify It Works
```bash
# Wait ~30 seconds for services to be ready
curl http://localhost:8080/healthz

# You should see: {"status":"healthy"}
```

🎉 **Genie is running!** Open http://localhost:8080 in your browser.

---

## Detailed Setup (for Development)

### Step 1: System Requirements

**macOS**
```bash
# Install Homebrew if not already installed
/bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"

# Install required tools
brew install go git docker
```

**Ubuntu/Debian**
```bash
# Install Go
wget https://go.dev/dl/go1.25.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.25.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
```

**Windows (WSL2 recommended)**
```bash
# In WSL2 terminal
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh

# Download Go from https://go.dev/dl/
# Or use Chocolatey: choco install golang docker-desktop
```

### Step 2: Clone Repository
```bash
git clone https://github.com/c2siorg/genie.git
cd genie

# Verify Go installation
go version  # Should show 1.25.0+
```

### Step 3: Environment Setup
```bash
# Copy example environment
cp .env.example .env

# Review and customize if needed (defaults work for local development)
# The .env file contains:
# - JWT secret (use any random string for local development)
# - Database connection string (will use docker postgres)
# - LLM configuration (use "mock" for fast development)
```

### Step 4: Start Services
```bash
# Start all services (PostgreSQL, Ollama, API, etc.)
docker-compose up -d

# Check status
docker-compose ps

# Expected output:
# NAME              STATUS       PORTS
# genie-api         Up 30s       0.0.0.0:8080->8080/tcp
# postgres          Up 30s       5432/tcp
# otel-collector    Up 30s       4317/tcp
```

**Wait 30-60 seconds for all services to be ready.**

### Step 5: Verify Setup
```bash
# Check API health
curl http://localhost:8080/healthz
# Response: {"status":"healthy"}

# Check database
docker-compose exec postgres psql -U genie -d genie -c "SELECT NOW();"
# Response: current_timestamp (if connection works)

# View logs
docker-compose logs genie-api
```

### Step 6: Test the API
```bash
# Create a user
curl -X POST http://localhost:8080/v1/users \
  -H 'Content-Type: application/json' \
  -d '{
    "email": "test@local.dev",
    "name": "Test User",
    "password": "TestPass123"
  }' | jq .

# Note the JWT token from response
TOKEN="your-jwt-token-here"

# Get current user (requires auth)
curl http://localhost:8080/v1/users/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```

---

## Development Workflow

### Running the Project Locally

**Option 1: Via Docker (Recommended)**
```bash
docker-compose up -d
# API runs inside container, automatically hot-reloads
```

**Option 2: Native Binary**
```bash
# Ensure services are running
docker-compose up -d postgres redis

# Build and run
go build -o genie ./cmd/api
./genie

# Or with hot reload (requires air)
go install github.com/cosmtrek/air@latest
air
```

### Building from Source
```bash
# Build binary
go build -o genie ./cmd/api

# Build Docker image
docker build -t genie-api:local .

# Run in Docker
docker run -p 8080:8080 \
  -e GENIE_JWT_SECRET=dev-secret \
  -e GENIE_LLM=mock \
  genie-api:local
```

### Running Tests
```bash
# All tests
go test ./...

# Specific package
go test ./pkg/payment -v

# With coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
# Opens coverage report in browser
```

### Useful Make Commands
```bash
make help           # Show all commands
make build          # Build the binary
make test           # Run tests
make test-coverage  # Generate coverage report
make lint           # Run linter
make fmt            # Format code
make up             # Start Docker stack
make down           # Stop Docker stack
make logs           # View logs
make clean          # Clean artifacts
```

---

## Project Structure

```
genie/
├── cmd/api/              # Main API server
│   └── main.go
├── pkg/                  # Go packages
│   ├── payment/          # Payment processing
│   ├── commerce/         # Commerce workflows
│   ├── compliance/       # Compliance checks
│   ├── cbdc/             # CBDC ledger integration
│   ├── merchant/         # Merchant onboarding
│   ├── web/              # HTTP handlers & UI
│   └── ...
├── docs/                 # Documentation
├── .github/              # GitHub workflows, templates
├── docker-compose.yaml   # Docker services
├── Makefile             # Make targets
├── go.mod              # Go module definition
├── README.md           # Main documentation
├── CONTRIBUTING.md     # Contribution guidelines
└── CODE_OF_CONDUCT.md # Community standards
```

---

## Common Tasks

### Add a New Dependency
```bash
go get github.com/user/package@v1.0.0
go mod tidy
```

### Connect to Database Directly
```bash
# Using Docker
docker-compose exec postgres psql -U genie -d genie

# Common queries:
\dt                    -- List tables
SELECT * FROM users;   -- View users
\q                     -- Exit
```

### View API Logs
```bash
# Stream logs from API container
docker-compose logs -f genie-api

# Or native logs
go run ./cmd/api 2>&1 | tee api.log
```

### Enable Debug Logging
```bash
# Set environment variable
export GENIE_LOG_LEVEL=debug

# Then run API
go run ./cmd/api
```

### Reset Development Environment
```bash
# Stop containers and remove volumes
docker-compose down -v

# Rebuild and start fresh
docker-compose up --build

# Reinitialize database
docker-compose exec genie-api /genie-api  # Triggers migrations
```

---

## Troubleshooting

### "connection refused" error
**Problem**: API can't connect to database

**Solution**:
```bash
# Check if PostgreSQL is running
docker-compose ps | grep postgres

# If not running, start it
docker-compose up postgres -d

# Wait for readiness (~10 seconds)
sleep 10

# Verify connection
docker-compose exec postgres psql -U genie -d genie -c "SELECT 1"
```

### "migration failed" error
**Problem**: Database schema migration failed

**Solution**:
```bash
# Check migration logs
docker-compose logs genie-api | grep -i migration

# Reset database and retry
docker-compose down -v  # Remove volumes
docker-compose up -d    # Start fresh

# Wait for auto-migration
sleep 30
curl http://localhost:8080/healthz
```

### Port already in use
**Problem**: Port 8080 already in use

**Solution**:
```bash
# Find process using port
lsof -i :8080

# Kill the process (if safe)
kill -9 <PID>

# Or change port in docker-compose.yaml
# Change "8080:8080" to "8081:8080"
docker-compose up -d
```

### Out of disk space (Docker)
**Problem**: Docker volumes consuming too much space

**Solution**:
```bash
# Clean up Docker resources
docker system prune -a --volumes

# Restart
docker-compose up -d
```

---

## Next Steps

After getting Genie running:

1. **Explore the API**: Try the endpoints documented in [API.md](./docs/api.md)
2. **Read the Code**: Start with [`cmd/api/main.go`](./cmd/api/main.go) to understand structure
3. **Run Tests**: Execute `go test ./...` to verify everything works
4. **Contribute**: See [CONTRIBUTING.md](./CONTRIBUTING.md) to get started
5. **Join Community**: Ask questions in [GitHub Discussions](https://github.com/c2siorg/genie/discussions)

---

## Learning Resources

### About Genie
- [README.md](./README.md) — Project overview
- [CLAUDE.md](./CLAUDE.md) — Architecture and patterns
- [docs/](./docs/) — Detailed documentation

### About e-Rupee & CBDC
- [RBI Digital Rupee Documentation](https://www.rbi.org.in/)
- [e-Rupee Whitepaper](https://www.rbi.org.in/web/rbi/-/digital_rupee)

### Go Development
- [Effective Go](https://golang.org/doc/effective_go)
- [Go by Example](https://gobyexample.com/)
- [Uber Go Style Guide](https://github.com/uber-go/guide)

---

## Getting Help

- **Documentation**: [docs/](./docs/) directory
- **API Reference**: [docs/api.md](./docs/api.md)
- **Issues**: [GitHub Issues](https://github.com/c2siorg/genie/issues)
- **Discussions**: [GitHub Discussions](https://github.com/c2siorg/genie/discussions)
- **Email**: info@c2si.org

---

## Key Points to Remember

✅ **Do**
- Use Docker Compose for development
- Run tests before committing
- Follow [CONTRIBUTING.md](./CONTRIBUTING.md)
- Ask questions in Discussions

❌ **Don't**
- Commit `.env` or secrets
- Modify migrations manually
- Commit database dumps
- Force push to main

---

**Happy developing!** 🚀

Last Updated: June 1, 2026
