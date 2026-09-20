# Module Specifications

## Overview

Three domain modules are implemented as concrete service structs under `internal/services/`: Population (`population.Service`), Resources (`resources.Service`) and Facilities (`facilities.Service`). No Go interfaces are declared for them; callers hold `*Service` pointers. Each service owns its repositories and an ID generator, and is constructed from a bare `*sql.DB` (`internal/services/population/service.go`, `internal/services/resources/service.go`, `internal/services/facilities/service.go`).

Two call sites construct the services: the TUI (`internal/tui/app.go`, `New`) and the simulation engine (`internal/simulation/engine.go`, `New`); each builds its own set over the same database handle. The simulation engine drives most of the write paths described below; the TUI drives resident CRUD and read-only listings.

Labor, Medical, Security and Governance have no service, repository or view code. Their design notes have been moved to [ROADMAP.md](ROADMAP.md).

As of commit `34c3398` the three service packages have no test files and report 0.0% statement coverage (`go test -race -count=1 -coverprofile=... ./internal/...`); repository coverage is 44.0%, models 83.5%.

Conventions shared by all three services:

- Repository methods that take `tx *sql.Tx` accept `nil` and then execute on the pool connection (`internal/repository/resource_repo.go`, `getExecer`).
- IDs come from `util.IDGenerator.NewID()`, a UUIDv7-style value whose low 62 bits are drawn from `crypto/rand` (`internal/util/id.go`).
- The database pool is opened with `SetMaxOpenConns(1)` (`internal/database/database.go`), so every statement from every caller serialises through one SQLite connection.

---

## Module: Population Registry

**Package:** `internal/services/population` (`service.go`, `genetics.go`, `demographics.go`)

**Responsibilities:** resident records, birth and death registration, household records, lineage lookups and the coefficient-of-inbreeding calculation, and demographic summaries (age/sex distribution, projection, workforce).

### Constructor

```go
func NewService(db *sql.DB, vaultNumber int) *Service
```

`NewService` builds a `ResidentRepository` and `HouseholdRepository` over `db`, an `IDGenerator`, and a `util.RegistryNumberGenerator` seeded with `vaultNumber` (`service.go`, `NewService`). The `regNumGen` field is stored but never called; registry numbers come from the repository's `GetNextRegistryNumber` instead (`service.go`, `CreateResident`, `RegisterBirth`).

### Exported methods

| Method | What it does | Writes | Errors |
| --- | --- | --- | --- |
| `CreateResident(ctx, input CreateResidentInput) (*models.Resident, error)` | Generates an ID and the next registry number, floors `ClearanceLevel` at 1 (no ceiling), sets `Status=ACTIVE`, inserts with `tx=nil`. | `residents` (1 INSERT) | `"generating registry number: %w"`, `"creating resident: %w"` |
| `GetResident(ctx, id) (*models.Resident, error)` | Pass-through to `ResidentRepository.GetByID`. | none | repository error unwrapped |
| `GetResidentByRegistryNumber(ctx, regNum) (*models.Resident, error)` | Pass-through to `GetByRegistryNumber`. | none | repository error unwrapped |
| `UpdateResident(ctx, id, input UpdateResidentInput) (*models.Resident, error)` | Read-modify-write: fetches the resident, overlays the non-nil fields of `UpdateResidentInput` (`Surname`, `GivenNames`, `BloodType`, `Status`, `DateOfDeath`, `HouseholdID`, `QuartersID`, `VocationID`, `ClearanceLevel`, `Notes`), updates with `tx=nil`. `Sex`, `DateOfBirth` and `EntryType` are not fields of the input and cannot be changed by this method. | `residents` (1 UPDATE) | `GetByID` error unwrapped, `"updating resident: %w"` |
| `ListResidents(ctx, filter models.ResidentFilter, page models.Pagination) (*models.ResidentList, error)` | Pass-through to `ResidentRepository.List`. | none | repository error |
| `RegisterBirth(ctx, input BirthRegistration) (*models.Resident, error)` | See "Birth registration" below. | `residents` (1 INSERT, in a transaction) | `"parent 1 not found: %w"`, `"parent 1 is deceased"`, same for parent 2, `"starting transaction: %w"`, `"generating registry number: %w"`, `"creating resident: %w"`, `"committing transaction: %w"` |
| `RegisterDeath(ctx, residentID, input DeathRegistration) error` | Fetches the resident, rejects if `!IsAlive()`, sets `Status=DECEASED` and `DateOfDeath`, appends `"Cause of death: <cause>"` to `Notes` when `Cause != ""` (preceded by a newline if notes already exist), updates with `tx=nil`. | `residents` (1 UPDATE) | `GetByID` error unwrapped, `"resident is already deceased"`, repository update error unwrapped |
| `CreateHousehold(ctx, input CreateHouseholdInput) (*models.Household, error)` | Generates an ID and the next designation, sets `Status=ACTIVE`, inserts with `tx=nil`. | `households` (1 INSERT) | `"generating designation: %w"`, `"creating household: %w"` |
| `GetHousehold(ctx, id)` / `ListHouseholds(ctx, filter, page)` / `GetHouseholdMembers(ctx, householdID)` | Pass-throughs to `HouseholdRepository.GetByID`, `HouseholdRepository.List`, `ResidentRepository.GetByHousehold`. | none | repository errors |
| `AssignToHousehold(ctx, residentID, householdID) error` | Fetches the resident, checks the household exists with a separate `GetByID`, sets `HouseholdID`, updates with `tx=nil`; no transaction spans the check and the write. | `residents` (1 UPDATE) | resident lookup error unwrapped, `"household not found: %w"` |
| `GetChildren(ctx, residentID)` / `GetParents(ctx, residentID)` | Pass-throughs to the repository. | none | repository errors |
| `GetPopulationStats(ctx) (*PopulationStats, error)` | Calls `CountByStatus` and fills `TotalActive`, `TotalDeceased`, `TotalExiled`, `OnMission`, `Quarantined`; `Total` is the sum of those five. | none | repository error unwrapped |
| `CalculateCOI(ctx, parent1ID, parent2ID) (float64, error)` | See "Coefficient of inbreeding" below. | none | `"getting ancestors for parent 1: %w"`, `"getting ancestors for parent 2: %w"` |
| `GetAncestry(ctx, residentID, generations) (*FamilyTree, error)` | Builds `FamilyTree.Ancestors` by recursive parent lookup up to `generations` levels; a parent that fails to load is skipped silently. | none | root lookup error unwrapped |
| `GetDescendants(ctx, residentID, generations) (*FamilyTree, error)` | Builds `FamilyTree.Descendants` via `GetChildren` recursively up to `generations` levels. | none | root or `GetChildren` error unwrapped |
| `FindCommonAncestors(ctx, id1, id2) ([]*models.Resident, error)` | Intersects the two five-generation ancestor maps and loads each common ancestor. | none | ancestor-map errors unwrapped |
| `GetAgeDistribution(ctx, asOf) (*AgeDistribution, error)` | See "Demographics". | none | `List` error unwrapped |
| `GetSexDistribution(ctx) (*SexDistribution, error)` | See "Demographics". | none | `List` error unwrapped |
| `ProjectPopulation(ctx, asOf, years) (*PopulationProjection, error)` | See "Demographics". | none | errors from the three stats calls unwrapped |
| `GetWorkforceStats(ctx, asOf) (*WorkforceStats, error)` | See "Demographics". | none | `List` error unwrapped |
| `AssessCOIRisk(coi float64) COIRiskLevel` (package func) | Buckets a COI value; see below. | none | none |

Input types: `CreateResidentInput`, `UpdateResidentInput`, `BirthRegistration`, `DeathRegistration`, `CreateHouseholdInput` (`service.go`). Result types: `PopulationStats` (`service.go`); `FamilyTree`, `FamilyTreeNode`, `COIRiskLevel` (`genetics.go`); `AgeDistribution`, `SexDistribution`, `PopulationProjection`, `ProjectionPoint`, `ViabilityAssessment`, `WorkforceStats` (`demographics.go`).

### Registry numbers and designations

`ResidentRepository.GetNextRegistryNumber` selects the lexically greatest `registry_number` (`ORDER BY registry_number DESC LIMIT 1`), parses it as `V%03d-%05d` and returns the next integer; an empty table yields `V<vault>-00001`, and a value that does not parse falls back to `COUNT(*)+1` (`internal/repository/resident_repo.go`). `HouseholdRepository.GetNextDesignation` does the same with `H-%04d`, starting at `H-0001` (`internal/repository/household_repo.go`). Neither read is performed inside the caller's transaction and there is no retry on a UNIQUE collision.

> ⚠ See [#5](https://github.com/jstephenperry/vt-uos/issues/5).

### Birth registration

`RegisterBirth` (`service.go`):

1. Loads both parents and requires `IsAlive()` on each; `IsAlive` is `Status != DECEASED`, so QUARANTINE, EXILED and SURFACE_MISSION parents pass (`internal/models/resident.go`).
2. Calls `CalculateCOI` on the pair; when `err == nil && coi > 0.0625` it formats a warning string and assigns it to `_`. Nothing is logged, stored or returned.
   > ⚠ See [#22](https://github.com/jstephenperry/vt-uos/issues/22).
3. Opens `BeginTx` with a deferred `Rollback`, then calls `GetNextRegistryNumber`, which reads through the pool connection rather than the transaction (`internal/repository/resident_repo.go`).
4. Builds the resident with `EntryType=VAULT_BORN`, `EntryDate=DateOfBirth`, `Status=ACTIVE`, `ClearanceLevel=1`, both `BiologicalParent*ID` and `HouseholdID` set from the input, inserts inside the transaction and commits.

The only production caller is the simulation engine (`internal/simulation/demographics.go`, `registerVaultBirth`).

### Coefficient of inbreeding

`CalculateCOI` (`genetics.go`) implements Wright's path coefficient:

```plaintext
COI = Σ over common ancestors A of  0.5^(n1 + n2 + 1) × (1 + F_A)
```

As built:

- `n1`, `n2` are taken from `getAncestorMap(parentID, 5)`: a recursive walk starting at the parent's own parents at generation 1, capped at generation 5. The map excludes the parent itself, so a parent who is also an ancestor of the other parent is never treated as a common ancestor.
- `getAncestorMap` marks an id `visited` before recording its generation, so an ancestor reachable by two paths keeps the generation of whichever path the DFS reached first; the `generation < existing` check that follows is never reached for a repeat visit.
  > ⚠ See [#26](https://github.com/jstephenperry/vt-uos/issues/26).
- An ancestor id that fails `GetByID` ends that branch with `return nil`; the error is discarded (`genetics.go`, `getAncestorMap`).
- `F_A` is computed by `calculateSimpleCOI(A.parent1, A.parent2, 3)`, which intersects three-generation ancestor maps and sums `0.5^(g1+g2+1)` with no recursion; its error is discarded (`ancestorCOI, _ = ...`).
- `pow` is a hand-rolled integer power loop (`genetics.go`, `pow`).

`AssessCOIRisk` buckets: `<= 0` NONE, `<= 0.0156` LOW, `<= 0.0625` MODERATE, `<= 0.125` HIGH, otherwise CRITICAL (`genetics.go`). `AssessCOIRisk`, `GetAncestry`, `GetDescendants` and `FindCommonAncestors` have no callers outside the package; `CalculateCOI` is reached only from `RegisterBirth`.

### Demographics

All four demographic methods page through `ResidentRepository.List` with `Status=ACTIVE` and `PageSize=100` until `page.Page >= result.TotalPages` (`demographics.go`).

*Age distribution* (`GetAgeDistribution`): `Resident.Age(asOf)` is year difference minus one if the month/day has not yet been reached (`internal/models/resident.go`). Buckets: Infants `<= 2`, Children `<= 12`, Adolescents `<= 17`, YoungAdults `<= 25`, Adults `<= 45`, MiddleAged `<= 65`, Seniors otherwise. `AverageAge` is the mean; `MedianAge` uses `calculateMedian`, an O(n²) selection sort followed by the middle element (even counts average the two middle values).

*Sex distribution* (`GetSexDistribution`): counts `M` and `F`; `Total = Male + Female`; `MaleRatio = Male / Total` when `Total > 0`.

*Projection* (`ProjectPopulation(ctx, asOf, years)`):

```plaintext
annualBirths = int(Female × 0.4 × 0.08)
annualDeaths = int(Infants×0.01 + Children×0.001 + Adolescents×0.001
                 + YoungAdults×0.002 + Adults×0.003 + MiddleAged×0.01 + Seniors×0.05)
if annualDeaths < 1 && TotalActive > 50: annualDeaths = 1
GrowthRate   = (annualBirths − annualDeaths) / TotalActive × 100
for y in 1..years:
    births = annualBirths; deaths = annualDeaths
    if population < 100: births = int(births × population / 100)
    population = max(0, population + births − deaths)
    append ProjectionPoint{Year: asOf.Year()+y, Population, Births, Deaths, NetChange}
```

Rates are computed once from the starting census and reused for every year; the only per-year adjustment is the sub-100 birth damping (`demographics.go`, `ProjectPopulation`). One `ProjectionPoint` is produced per year for `1..years`; there are no fixed 5/10/25/50-year checkpoints. `GrowthRate` divides by `TotalActive` with no zero guard.

*Viability* (`assessViability`): `MinimumViable = 160`; `IsViable` is true when the current population is `>= 160` and no projected year falls below it; `YearsToMVP` is the index of the first year under 160. Concerns are appended for negative `GrowthRate`, `Seniors > YoungAdults + Adults`, `Children + Infants < Seniors`, and `MaleRatio` outside `[0.4, 0.6]`; recommendations for `GrowthRate < 0.5`, `Seniors > Adults/2`, and `current < 300`; each list gets a default string when otherwise empty.

*Workforce* (`GetWorkforceStats`): ages 16–17 count as `TrainingAge` and `WorkingAge`; 18–65 as `FullWorkforce` and `WorkingAge`; `>= 66` as `RetirementAge` and dependents; younger as dependents. `DependencyRatio = dependents / workers` when `workers > 0`.

No production code calls any of the four demographic methods; the simulation's population figures are computed by raw SQL in `internal/simulation/metrics.go`.

### Repository methods

`ResidentRepository` (`internal/repository/resident_repo.go`): `Create`, `GetByID`, `GetByRegistryNumber`, `Update`, `List`, `Delete`, `GetNextRegistryNumber`, `GetByHousehold`, `GetChildren`, `GetParents`, `CountByStatus`. `List` orders by `surname, given_names`; `GetByHousehold` has no status filter and returns deceased members too.

`HouseholdRepository` (`internal/repository/household_repo.go`): `Create`, `GetByID`, `GetByDesignation`, `Update`, `List`, `Delete`, `GetMemberCount`, `GetNextDesignation`, `CountByStatus`, `GetByRationClass`.

`Delete` on both repositories has no production caller.

### TUI

Views: `internal/tui/views/population/census.go` (`CensusView`) and `resident_form.go` (`ResidentForm`), reached with `F3` (`internal/tui/keys.go`). `internal/tui/app.go` calls `ListResidents` (`loadCensus`), `CreateResident` and `UpdateResident` (`saveResident`), and `RegisterDeath` (`registerDeath`). The edit path forwards only `Surname`, `GivenNames`, `BloodType`, `ClearanceLevel` and `Notes` to `UpdateResident` (`app.go`, `saveResident`). Pressing `d` on a living resident in the detail view dispatches `registerDeath` immediately with the fixed cause string `"Cause pending investigation"`; no confirmation step exists (`app.go`, `handlePopulationKeys`, `registerDeath`). The dashboard's population count is a raw `SELECT COUNT(*) FROM residents WHERE status = 'ACTIVE'` in `app.go` (`loadPopulation`), bypassing the service.

> ⚠ See [#34](https://github.com/jstephenperry/vt-uos/issues/34).

---

## Module: Resource Management

**Package:** `internal/services/resources` (`service.go`, `types.go`)

**Responsibilities:** resource categories, items and stock lots; a transaction ledger for every quantity change; expiry processing; runway forecasting; household ration requirement totals; inventory audit corrections.

### Constructor

```go
func NewService(db *sql.DB) *Service
```

Builds a `ResourceRepository`, and also a `HouseholdRepository` and `ResidentRepository` used by the rationing methods (`service.go`, `NewService`).

### Exported methods

| Method | What it does | Writes | Errors |
| --- | --- | --- | --- |
| `CreateCategory(ctx, CreateCategoryInput) (*models.ResourceCategory, error)` | New ID, INSERT with `tx=nil`. | `resource_categories` | `"creating category: %w"` |
| `GetCategory(ctx, id)` / `GetCategoryByCode(ctx, code)` / `ListCategories(ctx)` | Repository pass-throughs. | none | repository errors |
| `CreateItem(ctx, CreateItemInput) (*models.ResourceItem, error)` | New ID, INSERT with `tx=nil`. | `resource_items` | `"creating item: %w"` |
| `GetItem(ctx, id)` / `GetItemByCode(ctx, code)` / `ListItems(ctx, categoryID, page)` | Repository pass-throughs; `ListItems` orders by `item_code`. | none | repository errors |
| `CreateStock(ctx, CreateStockInput) (*models.ResourceStock, error)` | Inserts an `AVAILABLE` lot, then inserts a `PRODUCTION` transaction with reason `"Initial stock receipt"`; two separate `tx=nil` writes. | `resource_stocks`, `resource_transactions` | `"creating stock: %w"`, `"recording receipt transaction: %w"` |
| `GetStock(ctx, id)` / `ListStocks(ctx, filter, page)` | Repository pass-throughs. | none | repository errors |
| `AdjustStock(ctx, stockID, StockAdjustment) error` | See "Stock adjustment". | `resource_stocks`, `resource_transactions` | `"getting stock: %w"`, `"adjustment would result in negative quantity"`, `"updating stock: %w"`, `"recording transaction: %w"` |
| `RecordConsumption(ctx, ConsumptionInput) error` | See "Consumption order". | via `AdjustStock` | `"listing stocks: %w"`, `"consuming from stock <id>: %w"`, `"insufficient stock: %.2f units remaining"` |
| `RecordProduction(ctx, ProductionInput) (*models.ResourceStock, error)` | Inserts a new `AVAILABLE` lot with `ReceivedDate: time.Now()` (wall clock; `ProductionInput` has no received-date field), then a `PRODUCTION` transaction. | `resource_stocks`, `resource_transactions` | `"creating stock: %w"`, `"recording production transaction: %w"` |
| `GetTransactionHistory(ctx, filter, page) (*models.TransactionList, error)` | Pass-through to `ListTransactions`. | none | repository error |
| `GetExpiringItems(ctx, withinDays) ([]*models.ResourceStock, error)` | Pass-through to `GetExpiringStocks`. | none | repository error |
| `ProcessExpiredItems(ctx, now) (int, error)` | See "Expiration". | `resource_stocks`, `resource_transactions` | `"getting expired stocks: %w"` |
| `GetResourceRunway(ctx, itemID) (*models.RunwayProjection, error)` | See "Runway". | none | `"getting total stock: %w"`, `"getting item: %w"`, `"getting daily consumption: %w"` |
| `CalculateHouseholdAllocation(ctx, householdID) (*models.RationAllocation, error)` | See "Rationing". | none | `"getting household: %w"`, `"getting members: %w"` |
| `GetVaultDailyRequirements(ctx) (*models.DailyRequirements, error)` | See "Rationing". | none | `"listing households: %w"` |
| `PerformInventoryAudit(ctx, stockID, actualQty, auditorID) error` | See "Inventory audit". | `resource_stocks`, `resource_transactions` | `"getting stock: %w"`, `"updating stock: %w"`, `"recording audit transaction: %w"` |

Input types are in `types.go`: `CreateCategoryInput`, `CreateItemInput`, `CreateStockInput`, `StockAdjustment`, `ConsumptionInput`, `ProductionInput`.

None of the methods opens a database transaction; every stock write and its ledger row are separate statements (`service.go`).

> ⚠ See [#9](https://github.com/jstephenperry/vt-uos/issues/9).

### Ration classes

`RationClass.CalorieTarget()` and `WaterTarget()` return fixed per-class values (`internal/models/household.go`):

| Class | kcal/day | Water L/day |
| --- | --- | --- |
| MINIMAL | 1500 | 2.0 |
| STANDARD | 2000 | 3.0 |
| ENHANCED | 2500 | 3.5 |
| LABOR_INTENSIVE | 3000 | 4.0 |
| MEDICAL | 2000 | 3.0 |
| (unknown) | 2000 | 3.0 |

### Rationing

`CalculateHouseholdAllocation` loads the household and its members and returns `DailyCalories = CalorieTarget × len(members)` and `DailyWaterL = WaterTarget × len(members)`; `WeeklyItems` is never populated (`service.go`). `GetVaultDailyRequirements` lists `ACTIVE` households (`PageSize: 1000`), applies the same headcount formula per household, sums `TotalCalories`/`TotalWaterL`, and fills `ByHousehold`; a household whose member lookup fails is skipped with `continue`. Members are fetched with `GetByHousehold`, which has no status filter, so deceased and quarantined members count toward demand (`internal/repository/resident_repo.go`). Nothing enforces an allocation; the numbers feed the simulation's daily draw only (`internal/simulation/consumption.go`, `consumeDailyResources`).

> ⚠ See [#31](https://github.com/jstephenperry/vt-uos/issues/31).

### Consumption order

`RecordConsumption` (`service.go`):

1. Lists `AVAILABLE` lots for the item with `Pagination{Page: 1, PageSize: 100}`; lots beyond the first 100 are never considered.
2. `ListStocks` orders `ORDER BY s.expiration_date ASC NULLS LAST, s.received_date ASC` (`internal/repository/resource_repo.go`), so the draw is earliest-expiry first, falling back to receipt order for lots without an expiration date.
3. For each lot, `available = Quantity − QuantityReserved` (`internal/models/resource.go`, `AvailableQuantity`); lots with `available <= 0` are skipped; `consume = min(remaining, available)`.
4. Calls `AdjustStock` with `QuantityChange: -consume`, `Type: CONSUMPTION`, and the input's `Reason`/`AuthorizedBy`; an error returns immediately, leaving earlier lots already decremented.
5. After the loop, `remaining > 0` returns `"insufficient stock: %.2f units remaining"`; the partial draw is not rolled back.
   > ⚠ See [#28](https://github.com/jstephenperry/vt-uos/issues/28).

`ConsumptionInput.RelatedEntityType`/`RelatedEntityID` are accepted but not written by `RecordConsumption` (`service.go`, `types.go`).

### Stock adjustment

`AdjustStock`: `GetStock`, `newQty = Quantity + QuantityChange`; `newQty < 0` is rejected; `newQty == 0` sets `Status=DEPLETED`; `UpdateStock` then a transaction row with the caller's `Type`, `Quantity = QuantityChange`, `BalanceAfter = newQty` (`service.go`). The read and the write are not locked against concurrent adjusters.

### Expiration

`ProcessExpiredItems(ctx, now)`: candidates come from `GetExpiringStocks(ctx, 0)`, whose SQL is `expiration_date <= date('now', '+0 days') AND status = 'AVAILABLE'` — SQLite's wall-clock date, not the `now` argument (`internal/repository/resource_repo.go`). Each candidate with `now.After(ExpirationDate)` is set to `EXPIRED` via `UpdateStock` (a failed update is skipped with `continue`), then a `SPOILAGE` transaction with `Quantity: -stock.Quantity`, `BalanceAfter: 0`, `Reason: "Expired"` is inserted; the insert's error is not checked, and `count` increments regardless. The lot's `Quantity` is left unchanged; only its status changes.

> ⚠ See [#13](https://github.com/jstephenperry/vt-uos/issues/13) and [#22](https://github.com/jstephenperry/vt-uos/issues/22).

### Runway

`GetResourceRunway(ctx, itemID)` (`service.go`):

```plaintext
totalStock       = SUM(quantity − quantity_reserved) over AVAILABLE lots of the item
dailyConsumption = SUM(ABS(quantity)) of CONSUMPTION transactions
                   with timestamp >= date('now', '-30 days'), ÷ 30
if dailyConsumption > 0:
    DaysRemaining   = int(totalStock / dailyConsumption)
    ProjectedRunout = time.Now() + DaysRemaining days
    Status          = CRITICAL if < 7, WARNING if < 30, else OK
else:
    DaysRemaining = -1; Status = OK
```

`GetTotalStockByItem` and `GetDailyConsumption` are in `internal/repository/resource_repo.go`; transaction `timestamp` is stamped `time.Now().UTC()` at insert (`CreateTransaction`). Both the 30-day window and `ProjectedRunout` use real time, not vault time. No production code calls `GetResourceRunway`; the operations board computes runway separately in `internal/simulation/metrics.go` (see [SIMULATION.md](SIMULATION.md)).

### Inventory audit

`PerformInventoryAudit(ctx, stockID, actualQty, auditorID)`: `difference = actualQty − Quantity`. When zero, only `LastAuditDate` (`time.Now()`) and `LastAuditBy` are updated and no ledger row is written. Otherwise `Quantity = actualQty` (`DEPLETED` if zero), `UpdateStock`, then an `AUDIT_CORRECTION` transaction with `Quantity: difference`, `BalanceAfter: actualQty`, `Reason: "Inventory audit correction"`, `AuthorizedBy: &auditorID` (`service.go`). Negative `actualQty` is not rejected by the service.

### Repository methods

`ResourceRepository` (`internal/repository/resource_repo.go`): `CreateCategory`, `GetCategory`, `GetCategoryByCode`, `ListCategories`, `CreateItem`, `GetItem`, `GetItemByCode`, `ListItems`, `CreateStock`, `GetStock`, `UpdateStock`, `ListStocks`, `GetExpiringStocks`, `GetTotalStockByItem`, `CreateTransaction`, `ListTransactions`, `GetDailyConsumption`. `UpdateStock` writes only `quantity, quantity_reserved, status, last_audit_date, last_audit_by, updated_at`.

### TUI

View: `internal/tui/views/resources/inventory.go` (`InventoryView`), reached with `F4` (`internal/tui/keys.go`). `internal/tui/app.go` (`loadInventory`) calls `ListCategories` (when the view needs them) and `ListStocks`. The detail screen's help line reads `Esc:Back  a:Adjust  u:Audit` (`inventory.go`, `RenderDetail`) but `handleResourceKeys` handles only `esc` while a detail is open (`app.go`); `AdjustStock` and `PerformInventoryAudit` are not reachable from the TUI.

### Callers

The simulation engine is the only production caller of `GetVaultDailyRequirements`, `RecordConsumption`, `RecordProduction`, `ProcessExpiredItems`, `GetCategoryByCode`, `ListItems` and `ListCategories` outside the TUI (`internal/simulation/consumption.go`). `AdjustStock`, `GetTransactionHistory`, `GetExpiringItems`, `GetResourceRunway`, `CalculateHouseholdAllocation` and `PerformInventoryAudit` have no production caller.

---

## Module: Facility Operations

**Package:** `internal/services/facilities` (`service.go`)

**Responsibilities:** facility system records, aggregate facility statistics, and maintenance record creation/listing. Efficiency wear, status transitions, the maintenance sweep and facility alerts are implemented in the simulation engine, not in this service (`internal/simulation/degradation.go`, `internal/simulation/alerts.go`); they are summarised at the end of this section and detailed in [SIMULATION.md](SIMULATION.md).

### Constructor

```go
func NewService(db *sql.DB) *Service
```

Builds a `FacilityRepository` and an `IDGenerator` (`service.go`, `NewService`).

### Exported methods

| Method | What it does | Writes | Errors |
| --- | --- | --- | --- |
| `CreateSystem(ctx, CreateSystemInput) (*models.FacilitySystem, error)` | New ID; `MaintenanceIntervalDays < 1` becomes 90; `NextMaintenanceDue = InstallDate + interval days`; forces `Status=OPERATIONAL`, `EfficiencyPercent=100`, `TotalRuntimeHours=0`; INSERT with `tx=nil` (the repository runs `Validate()` first). | `facility_systems` | `"creating facility system: %w"` |
| `GetSystem(ctx, id)` / `GetSystemByCode(ctx, code)` | Pass-throughs to `GetByID`/`GetByCode`; a missing row surfaces as `"facility system not found"` without wrapping `sql.ErrNoRows` (`internal/repository/facility_repo.go`, `scanSystem`). | none | repository errors |
| `ListSystems(ctx, filter models.FacilitySystemFilter, page) (*models.FacilitySystemList, error)` | Pass-through to `List`. | none | repository error |
| `UpdateSystem(ctx, id, UpdateSystemInput) (*models.FacilitySystem, error)` | Read-modify-write over the non-nil fields `Name`, `Status`, `EfficiencyPercent`, `CurrentOutput`, `Notes`; `Update` with `tx=nil`. | `facility_systems` | `GetByID` error unwrapped, `"updating facility system: %w"` |
| `ScheduleMaintenance(ctx, ScheduleMaintenanceInput) (*models.MaintenanceRecord, error)` | Verifies the system exists, builds a record with `ScheduledDate`, `EstimatedHours`, `LeadTechnicianID`, and snapshots `SystemStatusBefore`/`EfficiencyBefore` from the current row; INSERT with `tx=nil`. | `maintenance_records` | `"system not found: %w"`, `"creating maintenance record: %w"` |
| `CompleteMaintenance(ctx, CompleteMaintenanceInput) error` | See below. | `facility_systems` | `"starting transaction: %w"`, `"listing maintenance records: %w"`, `"maintenance record not found: <id>"`, `"system not found: %w"`, `"updating system: %w"`, `"committing transaction: %w"` |
| `GetFacilityStats(ctx, asOf) (*models.FacilityStats, error)` | `CountByStatus`, `GetAverageEfficiency`, `GetOverdueCount(asOf)`; `TotalSystems` sums the five named statuses plus `DESTROYED`, which has no field of its own. | none | repository errors unwrapped |
| `ListMaintenanceRecords(ctx, filter, page) (*models.MaintenanceRecordList, error)` | Pass-through. | none | repository error |

Input types: `CreateSystemInput`, `UpdateSystemInput`, `ScheduleMaintenanceInput`, `CompleteMaintenanceInput` (`service.go`).

### CompleteMaintenance

`CompleteMaintenance` opens `BeginTx`, then locates the record by listing up to 1000 maintenance records with an empty filter (through the pool, not the transaction) and scanning for `RecordID`; a record outside that first page is reported as not found. It loads the system, sets `Status = NewStatus`, `EfficiencyPercent = NewEfficiency`, `LastMaintenanceDate = time.Now().UTC()`, `NextMaintenanceDue = now + MaintenanceIntervalDays`, updates inside the transaction and commits. `WorkPerformed`, `PartsConsumed`, `ActualHours`, `Outcome` and `Notes` are accepted and discarded; the `maintenance_records` row is never updated, and the repository has no method to update one (`internal/repository/facility_repo.go`).

`ScheduleMaintenance`, `CompleteMaintenance`, `UpdateSystem` and `ListMaintenanceRecords` have no production caller.

### Date storage

The repository writes `last_maintenance_date` and `next_maintenance_due` as `YYYY-MM-DD` (`internal/repository/household_repo.go`, `nullableTimePtr`, used by `facility_repo.go`) and reads them back with `time.Parse(time.DateOnly, ...)`, ignoring the parse error (`internal/repository/facility_repo.go`, `scanSystem`). `GetOverdueCount` compares the column against `asOf.Format(time.RFC3339)` as a string. The simulation's maintenance sweep writes the same columns as RFC3339 (`internal/simulation/degradation.go`, `updateSystemServiced`), which then parse to the zero time on read.

> ⚠ See [#11](https://github.com/jstephenperry/vt-uos/issues/11).

### Efficiency, degradation, maintenance and alerts (implemented in `internal/simulation`)

Constants (`internal/simulation/degradation.go`): `efficiencyDegradedBelow = 80.0`, `efficiencyFailedBelow = 15.0`, `maintenanceChancePerDay = 0.34`, `maintenanceRestore = 35.0`.

*Daily wear* (`degradeSystems`), per system whose status is OPERATIONAL or DEGRADED (`SystemStatus.IsOperational`, `internal/models/facility.go`):

```plaintext
baseDecay = efficiency_decay_rate × 100   (percent/day; 0.05 if the result is <= 0)
wear      = 2.2 if asOf > NextMaintenanceDue else 1.0
jitter    = 0.5 + rng.Float64()           (0.5..1.5)
newEff    = max(0, eff − baseDecay × wear × jitter)
status    = FAILED if newEff < 15; DEGRADED if newEff < 80; else OPERATIONAL
runtime  += 24 × newEff/100
output    = capacity_rating × newEff/100  (0 when no capacity)
```

Status is recomputed from efficiency alone each day, so a system left DEGRADED at `>= 80` by an equipment-fault incident returns to OPERATIONAL on the next pass (`degradation.go`, `degradeSystems`; `internal/simulation/incidents.go`, `incidentEquipmentFault`).

*Maintenance sweep* (`maintenanceSweep`): a system needs service when overdue, FAILED, or `EfficiencyPercent < 80`; it is serviced with probability 0.34 (0.6 if FAILED). Service sets `efficiency = min(100, eff + 35)`, `status = 'OPERATIONAL'`, `last_maintenance_date = asOf`, `next_maintenance_due = asOf + MaintenanceIntervalDays`, draws 1.0 unit from the `PARTS` category ignoring the result, and inserts a `maintenance_records` row (`outcome='COMPLETED'`, `system_status_after='OPERATIONAL'`, type CORRECTIVE if the system was FAILED else PREVENTIVE) by raw SQL. Status is not checked before servicing, so OFFLINE, MAINTENANCE or DESTROYED systems below 80% are also set OPERATIONAL.

*Alerts* (`internal/simulation/alerts.go`): `SYSTEM_FAILURES` (CRITICAL) when any system is FAILED; `CRIT_SYS_<code>` (CRITICAL) for each POWER/WATER/HVAC/WASTE/SECURITY system that is FAILED or OFFLINE; `SYSTEMS_DEGRADED` (WARNING) when any is DEGRADED; `MAINTENANCE_OVERDUE` (WARNING) when `GetOverdueCount > 0`. Criticality is `isCriticalCategory` in `internal/simulation/metrics.go`.

### Repository methods

`FacilityRepository` (`internal/repository/facility_repo.go`): `Create`, `GetByID`, `GetByCode`, `Update`, `Delete`, `List`, `CountByStatus`, `GetAverageEfficiency`, `GetOverdueCount`, `CreateMaintenanceRecord`, `ListMaintenanceRecords`. `List` supports `Category`, `Status`, `Sector`, `SearchTerm` (LIKE on name/code) and `OverdueOnly` (`next_maintenance_due < datetime('now')`, wall clock). `GetAverageEfficiency` averages OPERATIONAL and DEGRADED systems only.

### TUI

View: `internal/tui/views/facilities/systems.go` (`SystemsView`), reached with `F5` (`internal/tui/keys.go`). `internal/tui/app.go` (`loadSystems`) calls `ListSystems`. The detail help line reads `Esc:Back  m:Maintenance Log` (`systems.go`, `RenderDetail`) but `handleFacilityKeys` handles only `esc` in detail mode (`app.go`); no maintenance-log view exists.

### Other callers

`internal/simulation/systems_seed.go` (`EnsureCoreSystems`) creates 15 core systems through `CreateSystem` when `facility_systems` is empty. `internal/simulation/metrics.go` calls `GetFacilityStats` and `ListSystems`; `internal/simulation/incidents.go` calls `GetSystem`. The engine's own writes to `facility_systems` and `maintenance_records` bypass this service (`internal/simulation/degradation.go`).

---

## Module: Simulation

The simulation control core (`internal/simulation`) owns time progression, the daily processing pipeline (consumption, production, expiration, degradation, maintenance, incidents, demographics), alerts, event persistence, checkpointing and the `protocol.VaultState` snapshot consumed by the TUI, master server and web console. It is documented in [SIMULATION.md](SIMULATION.md).
