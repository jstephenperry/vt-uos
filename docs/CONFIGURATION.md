# Configuration & Setup

## vault.toml Configuration

```toml
[vault]
designation = "Vault 076"
number = 76
region = "Appalachia"
commissioned_date = "2076-10-23"
sealed_date = "2077-10-23T09:47:00Z"
designed_capacity = 500
vault_type = "control"  # control | experimental

[vault.location]
latitude = 39.6295
longitude = -79.9559
depth_meters = 100

[overseer]
initial_overseer_id = ""  # Set on first initialization

[experiment]
enabled = false
protocol_id = ""
protocol_name = ""
classification = "NONE"  # NONE | CONFIDENTIAL | SECRET | OVERSEER_ONLY

[simulation]
enabled = true
time_scale = 60.0          # 1 real minute = 1 game hour
auto_events = true
event_frequency = "normal"  # minimal | reduced | normal | increased | chaotic
start_date = "2077-10-23T09:47:00Z"  # Vault seal date

[simulation.consumption]
calorie_variance = 0.1     # ±10% random variance
water_variance = 0.1
efficiency_decay_rate = 0.001  # % per day for systems

[display]
color_scheme = "green_phosphor"  # green_phosphor | amber | white
scan_lines = true
flicker = false
date_format = "2006-01-02"
time_format = "15:04:05"

[logging]
level = "info"  # debug | info | warn | error
file = "logs/vtuos.log"
max_size_mb = 10
max_backups = 5

[database]
path = "vault.db"
backup_interval_hours = 24
backup_retention_days = 30
```

## Environment Variables

| Variable | Description | Default |
| -------- | ----------- | ------- |
| `VTUOS_CONFIG` | Path to vault.toml | `./vault.toml` |
| `VTUOS_DB` | Database path override | From config |
| `VTUOS_LOG_LEVEL` | Log level override | From config |
| `VTUOS_NO_COLOR` | Disable color output | `false` |

## Installation

### From Source

```bash
# Clone repository
git clone https://github.com/yourusername/vt-uos.git
cd vt-uos

# Build
make build

# Run
./bin/vtuos
```

### Cross-Compilation

```bash
# For Raspberry Pi 4/5 (ARM64)
make build-linux-arm64

# For Raspberry Pi Zero 2W (ARM64)
make build-linux-arm64

# For development on macOS
make build-darwin-amd64

# For development on Windows
make build-windows-amd64
```

## First Run

On first launch, VT-UOS will:

1. Create database if it doesn't exist
2. Run all migrations
3. Prompt for initial vault setup:
   - Vault designation
   - Starting population
   - Initial resource stocks
   - Facility configuration
4. Generate sample data (optional)
5. Create initial Overseer account

## Database Management

### Migrations

Migrations are embedded in the binary and run automatically on startup.

```bash
# Check migration status
./vtuos migrate status

# Force migration to specific version
./vtuos migrate up 5

# Rollback
./vtuos migrate down 1
```

### Backup

```bash
# Manual backup
./vtuos backup

# Restore from backup
./vtuos restore backup-2077-10-23.db
```

### Reset

```bash
# Reset database (WARNING: Destructive)
./vtuos reset --confirm
```

## Configuration Loading Priority

1. Command-line flags
2. Environment variables
3. Configuration file
4. Defaults

## Logging

Logs are written to:
- File: `logs/vtuos.log` (rotated)
- Stderr: Errors only
- Structured JSON format for parsing

Example log entry:

```json
{
  "time": "2077-10-23T09:47:00Z",
  "level": "INFO",
  "msg": "Vault initialized",
  "vault": "V076",
  "population": 500,
  "version": "1.0.0"
}
```

## Performance Tuning

### SQLite Optimization

In `internal/database/database.go`:

```sql
PRAGMA journal_mode = WAL;          -- Write-Ahead Logging
PRAGMA synchronous = NORMAL;        -- Balance safety/speed
PRAGMA cache_size = -64000;         -- 64MB cache
PRAGMA temp_store = MEMORY;         -- Temp tables in RAM
PRAGMA mmap_size = 268435456;       -- 256MB memory-mapped I/O
```

### Memory Limits

For Raspberry Pi Zero 2W (512MB RAM):

```toml
[database]
cache_size_mb = 32
max_connections = 1

[simulation]
batch_size = 100
event_buffer = 50
```

## Security Considerations

1. **No Authentication** - This is a single-user system simulation
2. **No Encryption** - Database is plaintext SQLite
3. **No Network** - Offline-first, no remote access
4. **Audit Logging** - All changes logged to `audit_log` table

If deploying as multi-user:
- Add authentication layer
- Encrypt database with SQLCipher
- Implement role-based access control (RBAC)

## Glossary

| Term | Definition |
| ---- | ---------- |
| Sealed Date | When vault was closed, starting simulation clock |
| Game Time | Internal vault time (may run faster than real time) |
| Time Scale | Ratio of real time to game time (60:1 = 1 min real = 1 hour game) |
| Runway | Days until resource depletion at current consumption rate |
| MTBF | Mean Time Between Failures |
| COI | Coefficient of Inbreeding |
