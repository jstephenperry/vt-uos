# Terminal UI

The local operator console is a single Bubble Tea model, `tui.App` (`internal/tui/app.go`), started by `tui.Run` or `tui.RunWithEngine`, which runs it in the alternate screen and quits it when the context is cancelled.
`New` builds the population, resources, and facilities services directly from `db.DB`, the three list views, the simulation control view, the theme (from `[display] color_scheme`), and the default key map, and seeds boot/kiosk state from `[exhibit]`.
`AttachEngine` stores the simulation engine and primes the live snapshot; without an engine the dashboard and simulation views fall back to static content.
`Init` enters the alt screen, starts a 1-second tick, loads the population count, and (when `boot_sequence` is on) starts a 150 ms boot tick.
On every tick the app pushes the vault clock into the three views, refreshes the live snapshot and engine alerts when an engine is attached, runs attract-mode logic, and advances the alert bar every third tick.
The population count in the header comes from a raw `SELECT COUNT(*) FROM residents WHERE status = 'ACTIVE'` issued by the TUI itself, bypassing the repository and service layers; errors yield 0.

> ⚠ See [#45](https://github.com/jstephenperry/vt-uos/issues/45).

This document describes what renders today. Design intent for screens that do not exist yet has been moved to [ROADMAP.md](ROADMAP.md).

## Screen structure

`View` returns `Initializing VT-UOS...` until the first `WindowSizeMsg`, a shutdown line while quitting, the boot screen while booting, and otherwise header, alert bar, content, and footer.

```
VAULT-TEC UNIFIED OPERATING SYSTEM vX        Vault 076 │ POP: 500     ← header (renderHeader)
═══════════════════════════════════════════════════════════════════   ← double rule
2077-10-23 09:47:00 │ All systems operational                        ← alert bar (renderAlertBar)
                                                                      ← content, clamped to
                    [ module content ]                                  ContentHeight(height, 6)
───────────────────────────────────────────────────────────────────   ← single rule
[F1]Help [F2]Dashboard [F3]Population [F4]Resources ... [F10]Quit     ← footer (renderFooter)
```

- Header: title plus `v<Version>` on the left, `<designation> │ POP: <n>` on the right, followed by a double rule.
- Alert bar: vault time formatted with `[display] date_format` and `time_format`, a divider, then the current alert prefixed `INFO:`/`WARNING:`/`CRITICAL:` (or `All systems operational`), truncated to the remaining width.
- Content: the current module's render, wrapped in a lipgloss style with `Width(width)`, `Height`, and `MaxHeight` set to `ContentHeight(height, chromeLines)` where `chromeLines = 6`.
- Footer: a single rule and `KeyMap.StatusBarHelpResponsive(width)`, or `ATTRACT MODE — press any key to take control` while attract mode is active.
- `WindowSizeMsg` recomputes the visible rows of the census, inventory, and systems tables as `ContentHeight(height, 6) - 6`, floored at 5.

Alerts shown in the bar come from two lists: UI-raised alerts (`AddAlert`, newest first, capped at 10) followed by unacknowledged engine alerts mirrored by `syncAlertsFromEngine`; `AddAlert` resets the rotation to show the new alert immediately.

## Modules and function keys

| Key | Module | What happens |
| --- | --- | --- |
| `F1` | Help | remembers the current module and opens Help |
| `F2` | Dashboard | switches and clears any detail view |
| `F3` | Population | switches, clears detail, runs `loadCensus()` |
| `F4` | Resources | switches, clears detail, runs `loadInventory()` |
| `F5` | Facilities | switches, clears detail, runs `loadSystems()` |
| `F6` `F7` `F8` `F9` | Labor, Medical, Security, Governance | switches to a static screen (see below) |
| `F10` | — | quit (handled by `IsQuit` before the function-key switch; the `"quit"` case inside the switch is unreachable) |
| `F11` | Simulation | switches and clears detail |

`ModuleSettings` is declared but no key or code path ever selects it.
Labor, Medical, Security, and Governance render fixed literal figures (for example a shift roster of 165/180, `467` healthy residents, six security zones, five directives) under a `— monitoring mode` note; none of it is read from the database or the engine.

> ⚠ See [#23](https://github.com/jstephenperry/vt-uos/issues/23).

## Global key bindings

`handleKeyPress` evaluates keys in this order: boot screen → attract mode → quit-confirm dialog → resident form → census search → quit keys → function keys → Back → module handler.

| Key(s) | Condition | Action |
| --- | --- | --- |
| any key | boot screen showing | dismisses the boot screen; key is swallowed |
| any key | attract mode active | exits attract mode; key is swallowed |
| `y` `Y` `enter` | quit dialog open | quits |
| `n` `N` `esc` | quit dialog open | closes the dialog; every other key is swallowed |
| `q` `ctrl+c` `F10` | not kiosk | opens the CONFIRM EXIT dialog (`KeyMap.IsQuit`) |
| `q` `F10` | kiosk | INFO alert `Kiosk mode active — exit is disabled`; no dialog |
| `ctrl+c` | kiosk | still opens the CONFIRM EXIT dialog |
| `esc` `backspace` | a detail view is open | closes the detail view (`KeyMap.Back`) |
| `esc` `backspace` | Help open | returns to the module Help was opened from |

Bindings that exist in `DefaultKeyMap` (`internal/tui/keys.go`) but are never consulted by `handleKeyPress`: `Left` (`left`/`h`), `Right` (`right`/`l`), `Home` (`home`/`g`), `End` (`end`/`G`), `ctrl+u`/`ctrl+d` page aliases, `Help` (`?`), `Select` (`space`), `Tab`, `ShiftTab`, `Delete`.
`?` therefore does nothing at the shell level; `h`/`l` reach the Select component inside the resident form, and `h` is "step 1 hour" in Simulation Control.
`IsNavigation` and `StatusBarHelp` exist on `KeyMap` but are unused by the app, which calls `IsQuit`, `IsFunctionKey`, `GetFunctionKeyModule`, `Back.Matches`, and `StatusBarHelpResponsive`.

Footer help text by width (`StatusBarHelpResponsive`):

| Width | Footer |
| --- | --- |
| `< 60` | `[F1]? [F2]Dash [F3]Pop [F4]Res [F10]Quit` |
| `< 100` | `[F1]Help [F2]Dashboard [F3]Population [F4]Resources [F10]Quit` |
| otherwise | `[F1]Help [F2]Dashboard [F3]Population [F4]Resources [F5]Facilities [F6]Labor [F7]Medical [F8]Security [F9]Governance [F11]Sim [F10]Quit` |

## Views

### Dashboard (F2)

Purpose: four-panel status overview titled `═══ VAULT STATUS OVERVIEW ═══`.
Panels: POPULATION (active vs `[vault] designed_capacity` with a progress bar, plus births/deaths when live), CRITICAL SYSTEMS (up to five `critical` systems from the live `system_list`), RESOURCE STATUS (up to five resources with `daily_use > 0`, fill bar and runway), SIMULATION (status, time scale, vault time, elapsed; plus power balance, tick count, and an `[F11] open control core` hint when live).
Live data is used when `hasLive()` — an engine is attached and the snapshot's `schema_version` is non-empty; otherwise CRITICAL SYSTEMS and RESOURCE STATUS show hard-coded rows (Power 98 %, Water 95 %, HVAC 92 %, Security 100 %; Food 72 %/180 d, Water 85 %/240 d, Medical 60 %/120 d, Power 90 %/365 d).
The population ratio divides by `designed_capacity` with no zero guard.
Keys: none beyond the global set. Services: none directly; population count via `loadPopulation`, everything else from `engine.Snapshot()`.

### Census (F3, Population module)

Purpose: paginated resident list backed by `population.Service.ListResidents(ctx, filter, page)` with a `ResidentFilter` (search term, status) and `Pagination{Page, PageSize: 25}`.
Columns (`CensusView`): Registry #, Surname, Given Names, Age (right-aligned, computed against vault time), Sex, Blood, Status, Entry, Clr; lower-priority columns (Clr, Blood, Entry, Sex) drop first on narrow terminals.
Title `═══ POPULATION CENSUS ═══`; the current search term and status filter are echoed above the table; a failed load shows `Error: ...` and an empty page shows `No residents found.`.

| Key | View | Action |
| --- | --- | --- |
| `up`/`k`, `down`/`j` | list | move selection |
| `enter` | list | open Resident Details for the selection |
| `pgup` / `pgdown` | list | previous / next page, then `loadCensus()` |
| `a` | list | open the Resident form in Add mode |
| `/` or `s` | list | enter search mode |
| any single-byte char, `backspace` | search mode | edit the search buffer (shown as `SEARCH: <text>_`) |
| `enter` | search mode | apply the term (`SetSearch`, page reset to 1) and reload |
| `esc` | search mode | clear the term and reload |
| `esc` | detail | back to the list |
| `e` | detail | open the Resident form in Edit mode pre-filled from the selection |
| `d` | detail, resident alive | register a death immediately — no confirmation dialog |

Resident Details (`RenderDetail`) shows IDENTITY (registry #, name, sex, blood type), DATES (birth, age, entry type, entry date, death), STATUS (status, clearance, household id), and NOTES.
`d` calls `population.Service.RegisterDeath(ctx, id, DeathRegistration{DateOfDeath: clock.Now(), Cause: "Cause pending investigation"})`; on completion the detail closes, an alert is raised, and census and population count reload.
The status filter (`SetStatusFilter`) has no key binding.

### Resident form (Add / Edit)

Purpose: `popviews.ResidentForm` (`internal/tui/views/population/resident_form.go`) with ten fields in tab order: Surname*, Given Names*, Birth Year*, Month*, Day*, Sex (`M`/`F`), Blood Type (`A+`…`O-`, `-`), Entry Type (`ORIGINAL`/`VAULT_BORN`/`ADMITTED`), Clearance (default `1`, clamped 1–10 on read), Notes.
Title `═══ ADD RESIDENT ═══` or `═══ EDIT RESIDENT ═══`; the three date inputs render inline after a `Date of Birth:` (or `DOB:` under 60 columns) label.

| Key | Action |
| --- | --- |
| `tab` / `down` | next field (wraps) |
| `shift+tab` / `up` | previous field (wraps) |
| `enter` | next field, or submit on the last field |
| `ctrl+s` | submit |
| `esc` | cancel and close |
| other keys | forwarded to the focused `Input` or `Select` |

Submit validates Surname and Given Names as required and parses the date as `YYYY-MM-DD`; failures show `Invalid date of birth` or `Please fill in all required fields` and keep the form open.
On submit the app runs `saveResident()`: `population.Service.CreateResident` with `EntryDate = clock.Now()` for a new resident, or `UpdateResident` with surname, given names, blood type, clearance, and notes for an existing one; date of birth, sex, and entry type are not updated in Edit mode.
`saveResident`'s command reads the form (`GetData`) on a background goroutine while the form stays open to further keystrokes.

> ⚠ See [#34](https://github.com/jstephenperry/vt-uos/issues/34).

### Inventory (F4, Resources module)

Purpose: paginated stock list backed by `resources.Service.ListStocks(ctx, filter, page)` (`PageSize: 20`) plus one `ListCategories` call the first time, whose error is discarded.
Columns: Item Code, Name, Category, Quantity (right-aligned), Unit, Status, Expires (`EXPIRED`, `TODAY`, `<n>d` under 30 days, or the date).
Title `═══ RESOURCE INVENTORY ═══`; the active category is echoed above the table.

| Key | View | Action |
| --- | --- | --- |
| `up`/`k`, `down`/`j` | list | move selection |
| `enter` | list | open Stock Details |
| `pgup` / `pgdown` | list | previous / next page, then `loadInventory()` |
| `c` | list | set the category filter to the category after the selected stock's category (first category when nothing is selected, all categories after the last) and reload |
| `esc` | detail | back to the list |

Stock Details shows ITEM (code, name, unit, calories/unit), STOCK (quantity, reserved, available, status, location, lot), and DATES (received, expires with days remaining, last audit).
The detail help line advertises `a:Adjust  u:Audit`, but the resources key handler only recognises `esc` in detail view; `a` and `u` do nothing.

### Systems (F5, Facilities module)

Purpose: paginated facility list backed by `facilities.Service.ListSystems(ctx, filter, page)` (`PageSize: 25`).
Columns: Code, Name, Category, Status, Efficiency (right-aligned), Sector, Level, Maint Due.
Title `═══ FACILITY SYSTEMS ═══`; the active category is echoed above the table.

| Key | View | Action |
| --- | --- | --- |
| `up`/`k`, `down`/`j` | list | move selection |
| `enter` | list | open System Details |
| `pgup` / `pgdown` | list | previous / next page, then `loadSystems()` |
| `c` | list | cycle the category filter through POWER, WATER, HVAC, SECURITY, MEDICAL, FOOD_PRODUCTION, WASTE, COMMUNICATIONS, STRUCTURAL, then all, and reload |
| `esc` | detail | back to the list |

System Details shows IDENTIFICATION, STATUS (efficiency coloured below 80 % and 50 %), MAINTENANCE (overdue flag, MTBF), and NOTES.
The detail help line advertises `m:Maintenance Log`, but the facilities key handler only recognises `esc` in detail view.

### Simulation Control (F11)

Purpose: operator console for the simulation engine, rendered by `simviews.ControlView.Render` from the live snapshot, `engine.Events(12)`, and `engine.Alerts()`.
Without an engine it shows `The simulation control core is not attached in this mode.`.
Layout: status line (STATUS, SCALE — `frozen` at 0 — TICK, VAULT TIME, ELAPSED), a legend line, POPULATION & SYSTEMS beside POWER BALANCE and RESOURCE RUNWAY, ACTIVE ALERTS (first 5), OPERATIONAL EVENT LOG (first 8).

| Key | Engine call |
| --- | --- |
| `space` or `p` | `Pause()` if `Status() == RUNNING`, else `Resume()` |
| `+` or `=` | `SetTimeScale(next step up the ladder 1, 60, 600, 1440, 3600, 14400, 86400 from clock.TimeScale())`; the error is discarded |
| `-` or `_` | `SetTimeScale(next step down)`; the error is discarded |
| `s` | `Step(ctx, 24)` in a background command |
| `h` | `Step(ctx, 1)` in a background command |
| `k` | `CreateSnapshot(ctx)` in a background command; the path or error is raised as an alert |
| `a` | `AcknowledgeAlert(code)` for every alert in the live snapshot |

All keys are ignored when no engine is attached.
The step and snapshot commands use `context.Background()`, so quitting the TUI does not cancel them.

### Help (F1)

Purpose: static key reference titled `═══ HELP ═══` with a NAVIGATION list (F1–F9, F11, F10) and a CONTROLS list (Up/Down, Enter, Esc, `/`, Tab, PgUp/Dn, `a`, `e`, `d`, `c`), rendered in two columns at the Wide breakpoint and one column otherwise, ending with `Press Esc to return`.
`esc` or `backspace` returns to the module that was active when F1 was pressed.

### Boot screen, quit dialog, attract mode

The boot screen (`renderBoot`) reveals twelve fixed lines every 220 ms, shows `PRESS ANY KEY TO CONTINUE` when complete, and ends after 3.5 s or on any key.
The quit dialog (`renderConfirmDialog`) is a centred box reading CONFIRM EXIT / `Are you sure you want to exit?` / `[Y]es  [N]o` that replaces the content area.
Attract mode and kiosk behaviour are configured by `[exhibit]` and described in [DEPLOYMENT.md](DEPLOYMENT.md).

## Responsive breakpoints

`GetBreakpoint(width)` (`internal/tui/layout.go`) returns `BreakpointNarrow` below 60 columns, `BreakpointMedium` below 100, and `BreakpointWide` otherwise; the constant `BreakpointWide = 140` is a label, not a threshold.

| Element | Narrow (< 60) | Medium (60–99) | Wide (≥ 100) |
| --- | --- | --- | --- |
| Header | `VT-UOS` and `POP:<n>` | `VT-UOS v<ver>` | full title and version |
| Alert bar time | time only | date and time | date and time |
| Footer | short F1–F4/F10 | long F1–F4/F10 | all F-keys |
| Dashboard | panels stacked; bars 10 wide | two-by-two via `renderSideBySide`, which stacks anyway when the left panel is wider than `w/2 - 2` | same as Medium |
| Help | one column | one column | two columns |
| Census / Inventory / Systems help line | abbreviated | full | full |
| Detail label width | 14 (census, inventory), 16 (systems) | 18 (census), 20 (inventory, systems) | same |
| Form label width / help line | 10 / short | 16 / full | 16 / full |
| Tables | low-priority columns dropped until the row fits | fewer drops | weighted columns widen |

The dashboard width is floored at 40 and the header at 20 columns before layout.

## Components (`internal/tui/components`)

Only two files exist: `table.go` and `input.go`.

### Table

`Column` (`table.go`):

```go
type Column struct {
    Title    string
    Width    int              // fixed width, or minimum when Weight > 0
    Align    lipgloss.Position
    Sortable bool             // stored; no sorting is implemented
    Weight   float64          // proportional share of remaining width
    Priority int              // lower priority columns are dropped first
}
```

| Method | Behaviour |
| --- | --- |
| `NewTable(columns) *Table` | 10 visible rows, page size 25, default green styles |
| `SetRows([][]string)`, `RowCount()`, `Empty()` | replace / inspect data |
| `SetPagination(page, totalPages, totalRows)` | footer `Page p/n │ t total` when `totalPages > 0` |
| `SetVisibleRows(n)`, `SetStyles(header, row, rowAlt, selected, border)`, `Focus(bool)` | window size, styles, whether the selected row is highlighted |
| `Selected() int`, `SelectedRow() []string` | selection access |
| `MoveUp/MoveDown/PageUp/PageDown/GoToTop/GoToBottom` | clamp the selection and scroll the `offset` window |
| `Render()` / `RenderResponsive(width)` | header, rule, visible rows (alternating styles), optional pagination footer; `width <= 0` uses fixed widths |

`computeWidths` drops the lowest-`Priority` visible column until fixed widths plus 3-column separators plus 2 padding fit, then distributes the remainder by `Weight` with `Width` as the floor.
Cells are truncated and padded by byte length (`len(cell)`, `cell[:w-1]`), so multi-byte text misaligns or is split mid-rune.
No sorting exists despite the `Sortable` flag.

### Input

`NewInput(label)` creates a single-line text field (width 20, max length 100) with chainable `SetValue`, `SetPlaceholder`, `SetWidth`, `SetMaxLength`, `SetRequired`, `SetError`, plus `Focus(bool)`, `IsFocused()`, `Value()`, `HandleKey(key)`, `Validate() bool`, `Render()`, and `RenderWithLabelWidth(n)`.
`HandleKey` handles `backspace`, `delete`, `left`, `right`, `home`/`ctrl+a`, `end`/`ctrl+e`, and inserts any single-byte key up to `maxLength`; the cursor is a byte index.
`Validate` sets the error `Required` when a required field is blank after trimming.
Rendering shows the placeholder when empty and unfocused, an `_` cursor when focused, `*` after required labels, and uses hard-coded colour literals (`#00FF00`, `#66FF66`, `#FF4444`, `#006600`, `#00AA00`) rather than the `Theme`.

### Select

`NewSelect(label, options)` with `SetSelected(i)`, `Focus`, `IsFocused`, `Value()`, `SelectedIndex()`, `HandleKey` (`left`/`h` and `right`/`l` move without wrapping), `Render()`, `RenderWithLabelWidth(n)`; the selected option is shown as `[opt]` when focused and `(opt)` otherwise.

### Form

`NewForm(title)` holds `FormField` values (`Focus`, `IsFocused`, `HandleKey`, `Render`, `RenderWithLabelWidth`); `AddField` focuses the first field added.
`HandleKey`: `tab`/`down` next, `shift+tab`/`up` previous (both wrap), `ctrl+s` submit, `esc` cancel, `enter` next or submit on the last field, anything else to the focused field; `IsSubmitted`/`IsCancelled` flags are never reset.
`Render()`/`RenderResponsive(width)` draw `═══ <title> ═══`, the fields, an optional `Error: ...` line, and a help line; below 60 columns the label column is 10 and the help line is `Tab:Next  Ctrl+S:Save  Esc:Cancel`.
`ResidentForm` does not use `Form`; it re-implements the same navigation over the same `FormField` slice.

## Layout helpers (`internal/tui/layout.go`)

| Function | Behaviour | Used by |
| --- | --- | --- |
| `(*Theme).ProgressBar(value, max, width)` | `[███░░░]`, inner width at least 4; green above 60 %, warning above 30 %, error otherwise | dashboard, simulation view, client terminal |
| `Truncate(s, maxWidth)` | returns `s` if it fits; byte-slices when `maxWidth <= 3`, else rune-slices and appends `…` | alert bar, on an already-styled string |
| `PadRight`, `PadLeft` | pad by display width | — |
| `ContentWidth(term, min, max)`, `ContentHeight(term, chrome)` | clamp; height floors at 5 | view sizing |
| `CalculateColumnWidths(specs, width, sep)` | same drop-then-distribute algorithm over `ColumnSpec` | tests only |
| `SideBySide(left, right, total, gap)` | horizontal join or vertical fallback | nothing (the app uses its own `renderSideBySide`) |
| `(*Theme).Panel(title, content, width)` | rounded box with the title spliced into the top border by rune index | nothing |

## Style tokens (`internal/tui/styles.go`)

`NewTheme(scheme)` selects one of three palettes by `[display] color_scheme` (`green_phosphor` default, `amber`, `white`) and `buildTheme` derives every style from nine colours.

| Colour | green_phosphor | amber | white |
| --- | --- | --- | --- |
| Primary | `#00FF00` | `#FFAA00` | `#FFFFFF` |
| Secondary | `#00AA00` | `#AA7700` | `#AAAAAA` |
| Accent | `#66FF66` | `#FFCC66` | `#FFFFFF` |
| Background | `#000000` | `#000000` | `#000000` |
| Foreground | `#00FF00` | `#FFAA00` | `#FFFFFF` |
| Muted | `#006600` | `#664400` | `#666666` |
| Error | `#FF4444` | `#FF4444` | `#FF4444` |
| Warning | `#FFAA00` | `#FFFF00` | `#FFAA00` |
| Success | `#00FF00` | `#FFAA00` | `#00FF00` |

`Theme` fields: the nine `*Color` values; base styles `Base`, `Bold`, `Italic`, `Underline`; colour styles `Primary`, `Secondary`, `Accent`, `Error`, `Warning`, `Success`, `Muted`; component styles `Header` (primary, bold), `Footer` (secondary), `Title` (accent, bold), `Subtitle`, `Label`, `Value`, `Box` (rounded border), `Border`, `Selected` (inverse), `Focused`, `Disabled`, `Alert`, `AlertWarn`, `AlertCrit` (error colour, bold, blink); table styles `TableHeader`, `TableRow`, `TableRowAlt`; menu styles `MenuItem`, `MenuItemSelected`, `MenuItemDisabled`; form styles `FormLabel` (width 20), `FormInput`, `FormError`; status-bar styles `StatusBar` (background `#001100`), `StatusKey`, `StatusValue`, `StatusDivider` (` │ `).
Box-drawing constants `BoxTopLeft` … `BoxCross` (`┌ ┐ └ ┘ ─ │ ├ ┤ ┬ ┴ ┼`) and `BoxDoubleHorizontal`/`BoxDoubleVertical` (`═ ║`) are exported; `DrawBox`, `DrawHorizontalLine` (secondary colour), and `DrawDoubleLine` (primary colour) build rules from them.
The views under `internal/tui/views` and the components do not consume `Theme`; they instantiate their own lipgloss styles with the green-phosphor literals, so `amber` and `white` only recolour the shell (header, footer, alert bar, dashboard, help, placeholders) and the simulation view.
`[display] scan_lines` and `flicker` are parsed but not read by any rendering code.

## Client terminal rendering

The `vtuos connect` screen is a separate Bubble Tea model in `internal/client/view.go` that reuses `tui.NewTheme`, `DrawDoubleLine`, `DrawHorizontalLine`, and `ProgressBar`; its header reads `VT-UOS DISPLAY · <vault>` with a RUNNING/PAUSED/STOPPED badge, vault time, and `● LIVE`/`● OFFLINE`.
Its pages (dashboard, systems, resources, events, population) and keys are documented in [DEPLOYMENT.md](DEPLOYMENT.md).
