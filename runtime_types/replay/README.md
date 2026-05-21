# Replay Package

This package defines the data structures and interfaces for recording and replaying execution traces in Go-CCT.

## Trace Format

The execution trace is stored as a plain text file where each line represents a scheduling decision or a metadata event.

### 1. Random Seed
Records the seed used for the random number generator.

**Format:** `Seed: <int64>`

**Example:**
```
Seed: 1634567890123456789
```

### 2. Single Goroutine Event
Represents a scheduling decision where a single goroutine is woken up. This occurs for buffered channel operations, mutexes, waitgroups, etc.

**Format:** `LID[:CaseIndex]`

- **LID**: The Logical ID of the goroutine (e.g., `0`, `0.1`).
- **CaseIndex** (Optional): If the event corresponds to a `select` statement, this integer indicates which case was chosen. This is crucial for resolving non-determinism in `select`.

**Examples:**
- `0` (Goroutine 0 scheduled)
- `0.1:2` (Goroutine 0.1 scheduled, select case index 2 chosen)

### 3. Goroutine Pair Event (Rendezvous)
Represents a synchronization event involving two goroutines, typically an unbuffered channel send/receive.

**Format:** `SenderEvent,ReceiverEvent`

- **SenderEvent**: `LID[:CaseIndex]` for the sending goroutine.
- **ReceiverEvent**: `LID[:CaseIndex]` for the receiving goroutine.

**Examples:**
- `0,0.1` (Goroutine 0 sends to Goroutine 0.1)
- `0:1,0.1` (Goroutine 0 sends via select case 1 to Goroutine 0.1)

## Logical ID (LID)

To ensure deterministic replay across different runs where internal goroutine IDs (goid) might vary, Go-CCT uses Logical IDs.
- The main goroutine has LID `0`.
- A child goroutine inherits its parent's LID appended with a counter.
- Example: The first child of `0` is `0.0`, the second is `0.1`.
