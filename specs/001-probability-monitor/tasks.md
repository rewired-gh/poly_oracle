# Implementation Tasks: Event Probability Monitor

**Feature**: Event Probability Monitor
**Branch**: `001-probability-monitor`
**Date**: 2026-02-16

## Overview

This document provides a complete, sequential task breakdown for implementing the Event Probability Monitor service. Tasks are organized by user story to enable independent implementation and testing of each feature increment.

## Task Format

All tasks follow the strict checklist format:
```
- [ ] [TaskID] [P?] [Story?] Description with file path
```

Where:
- **TaskID**: Sequential identifier (T001, T002...)
- **[P]**: Parallelizable task marker (optional)
- **[Story]**: User story label (US1, US2, US3, US4) for story-specific tasks
- **Description**: Clear action with exact file path

---

## Phase 1: Project Setup ✅ COMPLETE

**Goal**: Initialize Go project with dependencies and basic structure

- [x] T001 Initialize Go module in go.mod
- [x] T002 Add dependency github.com/spf13/viper v1.19.0+ to go.mod
- [x] T003 Add dependency github.com/go-telegram-bot-api/telegram-bot-api v5.5.0+ to go.mod
- [x] T004 Create project directory structure per implementation plan
- [x] T005 Create Makefile with build, test, and run targets
- [x] T006 Create example configuration file in configs/config.yaml.example
- [x] T007 Create README.md with project overview and quick start instructions
- [x] T008 [P] Create .gitignore for Go project

---

## Phase 2: Foundational Components ✅ COMPLETE

**Goal**: Implement core models and infrastructure needed by all user stories

### Configuration Package

- [x] T009 Define Config structs in internal/config/config.go (PolymarketConfig, MonitorConfig, TelegramConfig, StorageConfig, LoggingConfig)
- [x] T010 Implement Load function in internal/config/config.go using Viper to read YAML
- [x] T011 Implement Validate function in internal/config/config.go with all validation rules
- [x] T012 Implement GetXxxConfig methods in internal/config/config.go
- [x] T013 [P] Write unit tests for config package in internal/config/config_test.go

### Models Package

- [x] T014 [P] Define Event entity in internal/models/event.go with all fields and validation
- [x] T015 [P] Define Snapshot entity in internal/models/snapshot.go with all fields and validation
- [x] T016 [P] Define Change entity in internal/models/change.go with all fields and validation
- [x] T017 [P] Implement Validate methods for all entities in internal/models/
- [x] T018 [P] Write unit tests for models package in internal/models/*_test.go

### Storage Package

- [x] T019 Define Storage struct in internal/storage/storage.go with in-memory maps and mutex
- [x] T020 Implement AddEvent, GetEvent, GetAllEvents, UpdateEvent in internal/storage/storage.go
- [x] T021 Implement AddSnapshot, GetSnapshots, GetSnapshotsInWindow in internal/storage/storage.go
- [x] T022 Implement AddChange, GetTopChanges, ClearChanges in internal/storage/storage.go
- [x] T023 Implement Save method in internal/storage/storage.go for JSON persistence
- [x] T024 Implement Load method in internal/storage/storage.go for JSON restoration
- [x] T025 Implement RotateSnapshots method in internal/storage/storage.go respecting max_snapshots_per_event
- [x] T026 Implement RotateEvents method in internal/storage/storage.go respecting max_events
- [x] T027 Implement thread-safety with sync.RWMutex in all Storage methods
- [x] T028 [P] Write unit tests for storage package in internal/storage/storage_test.go

---

## Phase 3: User Story 1 - Real-time Probability Change Alerts (P1) ✅ COMPLETE

**Story Goal**: Receive Telegram notifications when event probabilities change significantly

**Independent Test**: Configure an event with probability change exceeding threshold, verify notification received via Telegram within 2 minutes

**Priority**: P1 - Core value proposition, MVP functionality

### API Fix Tasks (CRITICAL - Real Polymarket API) ✅ COMPLETE

- [x] T076 Fix PolymarketClient to use real Gamma API endpoint (https://gamma-api.polymarket.com/events) in internal/polymarket/client.go
- [x] T077 Update PolymarketEvent struct with real API fields (title, volume24hr, volume1wk, volume1mo, markets array) in internal/polymarket/client.go
- [x] T078 Fix FetchEvents to parse direct array response (NOT wrapped in {events: []}) in internal/polymarket/client.go
- [x] T079 Implement multi-market handling: calculate max change across all markets per event in internal/polymarket/client.go
- [x] T080 Implement logical OR volume filtering (include if ANY volume field meets threshold) in internal/polymarket/client.go
- [x] T081 Update Event model to include Volume24hr, Volume1wk, Volume1mo, Liquidity fields in internal/models/event.go
- [x] T082 Update Config to include Volume24hrMin, Volume1wkMin, Volume1moMin thresholds in internal/config/config.go
- [x] T083 Update main.go to pass volume thresholds to FetchEvents in cmd/poly-oracle/main.go
- [x] T084 [P] Write integration tests for real Polymarket API in internal/polymarket/client_test.go
- [x] T085 Test with actual Polymarket API and verify filtering works correctly

### Configuration Tasks (NEW - User Requirements) ✅ COMPLETE

- [x] T086 Create sensible default config for hourly notifications in configs/config.yaml.example
- [x] T087 Configure event categories: tech, finance, geopolitics in configs/config.yaml.example
- [x] T088 Configure monitoring to produce 0-5 events per notification (avg 3) in configs/config.yaml.example
- [x] T089 Update main.go to pass volume thresholds to FetchEvents in cmd/poly-oracle/main.go

### Original Polymarket Client Tasks (COMPLETED BUT NEED FIXES)

- [x] T029 [US1] Define PolymarketClient interface in internal/polymarket/client.go
- [x] T030 [US1] Implement FetchEvents method in internal/polymarket/client.go using Gamma API
- [x] T031 [US1] Implement FetchMarketData method in internal/polymarket/client.go using CLOB API
- [x] T032 [US1] Implement retry logic with exponential backoff in internal/polymarket/client.go
- [x] T033 [US1] Implement Start/Stop methods for background polling in internal/polymarket/client.go
- [x] T034 [US1] [P] Write unit tests for polymarket package in internal/polymarket/client_test.go

### Monitor Package

- [x] T035 [US1] Define Monitor interface in internal/monitor/monitor.go
- [x] T036 [US1] Implement DetectChanges algorithm in internal/monitor/monitor.go (threshold + time window filtering)
- [x] T037 [US1] Implement RankChanges method in internal/monitor/monitor.go (sort by magnitude descending)
- [x] T038 [US1] Implement Start/Stop methods for monitoring cycle loop in internal/monitor/monitor.go
- [x] T039 [US1] [P] Write unit tests for monitor package in internal/monitor/monitor_test.go with table-driven tests

### Telegram Notifier

- [x] T040 [US1] Define Notifier interface in internal/notify/notifier.go
- [x] T041 [US1] Implement Telegram client in internal/telegram/client.go wrapping telegram-bot-api
- [x] T042 [US1] Implement Send method in internal/telegram/client.go with message formatting per contract
- [x] T043 [US1] Implement retry logic for Telegram API errors in internal/telegram/client.go
- [x] T044 [US1] [P] Write unit tests for telegram package in internal/telegram/client_test.go

### Main Application

- [x] T045 [US1] Implement service orchestration in cmd/poly-oracle/main.go (load config, initialize components)
- [x] T046 [US1] Implement graceful shutdown handling in cmd/poly-oracle/main.go
- [x] T047 [US1] Implement logging setup in cmd/poly-oracle/main.go (structured JSON logging)
- [x] T048 [US1] Wire all components together in cmd/poly-oracle/main.go (storage, polymarket client, monitor, notifier)
- [x] T049 [US1] Write integration test for end-to-end flow in tests/integration/e2e_test.go

---

## Phase 7: Polish & Cross-Cutting Concerns ✅ COMPLETE

**Goal**: Complete deployment configurations, documentation, and performance optimization

### Deployment Configurations

- [x] T063 [P] Create Dockerfile in deployments/Dockerfile with multi-stage build
- [x] T064 [P] Create systemd service unit file in deployments/systemd/poly-oracle.service
- [x] T065 [P] Create docker-compose.yml for easy local development
- [x] T066 [P] Add deployment instructions to README.md

### Performance & Reliability

- [x] T067 [P] Add performance benchmarks for change detection in internal/monitor/monitor_test.go
- [x] T068 [P] Add performance benchmarks for storage operations in internal/storage/storage_test.go
- [x] T069 [P] Implement health check endpoint in cmd/poly-oracle/main.go (optional)
- [x] T070 [P] Add graceful degradation for Polymarket API failures in internal/polymarket/client.go

### Documentation & Finalization

- [x] T071 Update README.md with complete setup and usage instructions
- [x] T072 Add inline code comments for complex algorithms (change detection)
- [x] T073 [P] Create example configuration scenarios in configs/ directory
- [x] T074 Verify all tests pass with go test ./...
- [x] T075 Verify linting passes with golangci-lint run

---

## Dependencies & Execution Strategy

### User Story Completion Order

```
Phase 1: Setup (blocking)
   ↓
Phase 2: Foundational (blocking)
   ↓
Phase 3: US1 - Real-time Alerts (P1) ← MVP DELIVERABLE
   ↓
Phase 4: US2 - Customizable Config (P2)
   ↓
Phase 5: US4 - Watchlist Management (P2)
   ↓
Phase 6: US3 - Multi-Channel Architecture (P3)
   ↓
Phase 7: Polish
```

### Parallel Execution Opportunities

**Within Phase 2 (Foundational)**:
- T013 (config tests), T018 (models tests), T028 (storage tests) can run in parallel after implementation tasks

**Within Phase 3 (US1)**:
- T034 (polymarket tests), T039 (monitor tests), T044 (telegram tests) can run in parallel after implementation tasks
- All test tasks marked with [P] can be parallelized

**Within Phase 7 (Polish)**:
- T063-T066 (deployment configs) can run in parallel
- T067-T068 (benchmarks) can run in parallel
- T071, T072, T073 (documentation) can run in parallel

### Independent Testing Strategy

Each user story phase includes:
1. Unit tests for individual components (marked with [P])
2. Integration tests for component interactions
3. Clear independent test criteria at story level

**US1 Independent Test**: End-to-end integration test (T049) validates complete flow from polling to notification delivery

**US2 Independent Test**: Configuration reload test (T053) validates parameter changes take effect immediately

**US4 Independent Test**: Watchlist filtering test (T057) validates only watched events trigger notifications

**US3 Independent Test**: Dispatcher test (T062) validates multi-channel architecture works correctly

---

## Task Summary

- **Total Tasks**: 75
- **Setup Phase**: 8 tasks
- **Foundational Phase**: 20 tasks
- **US1 (P1) Phase**: 21 tasks (MVP)
- **US2 (P2) Phase**: 4 tasks
- **US4 (P2) Phase**: 4 tasks
- **US3 (P3) Phase**: 5 tasks
- **Polish Phase**: 13 tasks

**Parallelizable Tasks**: 25 tasks marked with [P]

**Test Tasks**: 13 explicit test tasks + integration tests per story

---

## MVP Scope Recommendation

**Minimum Viable Product**: Complete Phase 1, Phase 2, and Phase 3 (US1)

This delivers:
- ✅ Polymarket API integration with automatic polling
- ✅ Probability change detection with configurable threshold
- ✅ Telegram notifications for top k events
- ✅ In-memory storage with persistence
- ✅ YAML configuration
- ✅ Basic deployment (binary + Docker + systemd)

**Estimated MVP Tasks**: 49 tasks (Phase 1 + Phase 2 + Phase 3)

---

## Implementation Notes

### Constitution Compliance

All tasks adhere to constitutional principles:
- **Simplicity**: Single-user mode, file-based config, in-memory storage
- **Go Language**: Idiomatic Go patterns, standard library preference
- **Robust Dependencies**: Only 2 external dependencies (Viper, Telegram Bot API)
- **Testing**: Comprehensive unit and integration tests for sustainable feedback loop
- **Code Quality**: Clear naming, explicit error handling, focused functions

### Testing Strategy

Per user requirement "write decent amount of tests (including unit tests)":
- Every package has unit tests (*_test.go files)
- Table-driven tests for multiple scenarios
- Integration tests for end-to-end validation
- Benchmarks for performance-critical paths (storage, change detection)

### Deployment Readiness

Tasks T063-T066 ensure deployment is "simple, elegant, and robust":
- Single binary executable
- Docker container with multi-stage build
- systemd service for daemon operation
- Clear documentation in README.md

---

## Next Steps

1. Execute tasks sequentially starting with T001
2. Complete Phase 1 and Phase 2 before starting user stories
3. Deliver MVP after completing Phase 3 (US1)
4. Gather user feedback before continuing with US2, US4, US3
5. Polish and optimize in Phase 7 before production deployment

**Suggested Command**: `/speckit.implement` to begin execution
