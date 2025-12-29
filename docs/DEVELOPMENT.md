# Development Guide

## Development Workflow

### Branch Strategy

- `main` - Stable, release-ready
- `develop` - Integration branch
- `feature/*` - Feature development
- `fix/*` - Bug fixes

### Commit Messages

Follow Conventional Commits:

```plaintext
feat(population): add COI calculation for lineage tracking
fix(resources): correct expiration date comparison
docs(readme): update installation instructions
refactor(tui): extract table component
test(simulation): add property tests for consumption
```

### Code Style

- Run `gofmt` and `goimports` on all code
- Use `golangci-lint` with provided config
- No exported functions without documentation comments
- Errors must be wrapped with context: `fmt.Errorf("loading resident %s: %w", id, err)`

### Pull Request Checklist

- [ ] Tests pass
- [ ] Lint passes
- [ ] Documentation updated
- [ ] Migration added (if schema change)
- [ ] CHANGELOG updated

## Testing Requirements

### Unit Tests

- All service methods must have unit tests
- Repository methods tested against in-memory SQLite
- Simulation calculations tested with known inputs/outputs
- Minimum 80% code coverage for `internal/services/`

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific package
go test ./internal/services/population/...
```

### Integration Tests

- Full CRUD workflows for each module
- Simulation engine tick processing
- Database migration up/down

```bash
# Run integration tests
make test-integration
```

### Property-Based Tests

- COI calculation for random family trees
- Resource consumption calculations
- Population projection accuracy

Example with `gopter`:

```go
func TestCOIProperties(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("COI is between 0 and 1", prop.ForAll(
        func(tree FamilyTree) bool {
            coi := CalculateCOI(tree.Parent1, tree.Parent2)
            return coi >= 0 && coi <= 1
        },
        genFamilyTree(),
    ))
    
    properties.TestingRun(t)
}
```

### Test Data

Located in `testdata/`:

- `vault-076.toml` - Test vault configuration
- `seed/day1/` - Day 1 vault with 500 residents
- `seed/year10/` - Year 10 vault with demographic changes
- `seed/crisis/` - Crisis scenarios (resource shortage, epidemic)

## Implementation Order

### Phase 1: Foundation (Week 1)

1. ✅ Project scaffolding, Go modules, Makefile
2. ✅ Configuration loading
3. ✅ Database connection, migration system
4. ✅ Base TUI application shell
5. ✅ Logging infrastructure

### Phase 2: Population Module - MVP (Week 2-3)

1. Resident model and repository
2. Household model and repository
3. Population service (CRUD operations)
4. Census TUI view (list, search, view detail)
5. Add/edit resident forms
6. Basic vital records (birth, death)

**Deliverable:** Can manage 500 residents with CRUD operations via TUI

### Phase 3: Resource Module (Week 4-5)

1. Resource models and repositories
2. Inventory management service
3. Transaction recording
4. Inventory TUI views
5. Expiration tracking

**Deliverable:** Can track resources and consumption

### Phase 4: Facility Module (Week 6)

1. Facility system models
2. Maintenance tracking
3. System status TUI views
4. Work order management

**Deliverable:** Can monitor vault systems

### Phase 5: Simulation Engine (Week 7-8)

1. Time management
2. Resource consumption loop
3. Population events (aging, births, deaths)
4. System degradation
5. Random event system

**Deliverable:** Vault operates autonomously over time

### Phase 6: Remaining Modules (Week 9-11)

1. Labor allocation (Week 9)
2. Medical records (Week 10)
3. Security and access control (Week 10)
4. Governance and directives (Week 11)

### Phase 7: Polish (Week 12)

1. Dashboard with key metrics
2. Alert system
3. Reporting and exports
4. Performance optimization
5. Documentation

## Architecture Guidelines

### Dependency Injection

Use constructor injection, no globals:

```go
type PopulationService struct {
    repo   repository.ResidentRepository
    logger *slog.Logger
}

func NewPopulationService(repo repository.ResidentRepository, logger *slog.Logger) *PopulationService {
    return &PopulationService{
        repo:   repo,
        logger: logger,
    }
}
```

### Error Handling

Always wrap errors with context:

```go
resident, err := s.repo.GetResident(ctx, id)
if err != nil {
    return nil, fmt.Errorf("population service: getting resident %s: %w", id, err)
}
```

### Transaction Management

Use repository methods that accept `*sql.Tx`:

```go
func (s *PopulationService) RegisterBirth(ctx context.Context, input BirthRegistration) (*Resident, error) {
    tx, err := s.db.BeginTx(ctx, nil)
    if err != nil {
        return nil, fmt.Errorf("starting transaction: %w", err)
    }
    defer tx.Rollback()
    
    // Multiple operations
    resident, err := s.repo.CreateResident(ctx, tx, input.Resident)
    if err != nil {
        return nil, err
    }
    
    err = s.repo.RecordVitalEvent(ctx, tx, input.VitalRecord)
    if err != nil {
        return nil, err
    }
    
    if err := tx.Commit(); err != nil {
        return nil, fmt.Errorf("committing transaction: %w", err)
    }
    
    return resident, nil
}
```

### Logging

Use structured logging with context:

```go
s.logger.Info("resident created",
    "resident_id", resident.ID,
    "registry_number", resident.RegistryNumber,
    "entry_type", resident.EntryType,
)

s.logger.Error("failed to create resident",
    "error", err,
    "input", input,
)
```

## Build System

### Makefile Targets

```bash
# Build
make build                  # Build for current platform
make build-linux-arm64      # Cross-compile for Pi
make build-all              # Build all platforms

# Test
make test                   # Run unit tests
make test-integration       # Run integration tests
make test-coverage          # Generate coverage report

# Quality
make lint                   # Run golangci-lint
make fmt                    # Format code
make vet                    # Run go vet

# Database
make migrate-up             # Run migrations
make migrate-down           # Rollback migrations
make migrate-status         # Check migration status

# Development
make run                    # Run application
make clean                  # Clean build artifacts
make deps                   # Download dependencies
```

### Example Makefile

```makefile
.PHONY: build test lint

BINARY_NAME=vtuos
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS=-ldflags "-X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME}"

build:
	go build ${LDFLAGS} -o bin/${BINARY_NAME} ./cmd/vtuos

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build ${LDFLAGS} -o bin/${BINARY_NAME}-linux-arm64 ./cmd/vtuos

test:
	go test -v -race -cover ./...

test-coverage:
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run

fmt:
	gofmt -s -w .
	goimports -w .

clean:
	rm -rf bin/
	rm -f coverage.out coverage.html
```

## AI Assistant Guidelines

When implementing features for this project:

1. **Start with the data model** - Ensure the SQLite schema supports the feature (see [DATABASE.md](DATABASE.md))
2. **Write the repository layer first** - Pure data access, no business logic
3. **Implement service layer with tests** - Business logic lives here (see [MODULES.md](MODULES.md))
4. **Build TUI last** - UI should call services, not repositories (see [TUI.md](TUI.md))
5. **Keep functions small** - Under 50 lines preferred
6. **Handle all errors** - No ignored errors, wrap with context
7. **Use transactions** - Multi-step database operations must be atomic
8. **Log meaningfully** - Debug for development, Info for operations, Warn/Error for problems
9. **Follow existing patterns** - Look at implemented modules for conventions
10. **Ask clarifying questions** - If a requirement is ambiguous, ask before assuming

## Performance Optimization

### Database Query Optimization

1. Use indexes on frequently queried columns
2. Use prepared statements for repeated queries
3. Batch inserts when possible
4. Use transactions for multiple operations
5. Profile slow queries with `EXPLAIN QUERY PLAN`

### Memory Optimization

1. Stream large result sets, don't load all into memory
2. Use pagination for UI lists
3. Release resources promptly (defer cleanup)
4. Profile with `pprof` to identify leaks

### TUI Optimization

1. Debounce rapid updates
2. Only re-render changed regions
3. Lazy-load data on demand
4. Cache computed values

## References

### Fallout Lore (for authenticity)

- Vault-Tec terminal entries from Fallout 3, 4, 76
- Vault Dweller's Survival Guide (in-game documents)
- Overseer logs and protocols

### Real-World Analogs

- HMIS (Homeless Management Information System) - population tracking
- Cold War civil defense shelter protocols
- Submarine crew management systems
- Space station life support monitoring
- Hospital information systems (for medical module)

### Technical References

- [Bubble Tea Documentation](https://github.com/charmbracelet/bubbletea)
- [Lip Gloss Styling](https://github.com/charmbracelet/lipgloss)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [Go Database Patterns](https://www.alexedwards.net/blog/organising-database-access)
