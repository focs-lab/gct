<a href="url"><img src="gct-logo.png" align="right" width="200" ></a>
 
 # GCT: Concurrency Testing for Go

GCT helps find concurrency bugs in Go programs by taking control of goroutine scheduling during tests. It can explore alternative schedules, record failures, and replay them deterministically, making flaky schedule-dependent bugs easier to reproduce and debug.

## Quick start

Install the `gct` command from this checkout:

```bash
go install ./cmd/gct
```

Make sure Go's command install directory is on your `PATH`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```

Instrument a target package:

```bash
gct instrument ./...
```

Run tests under GCT:

```bash
gct test ./...
```

Replay a recorded schedule:

```bash
gct replay trace.log
```

GCT currently uses `random_walk` as the default scheduler and writes schedules to `trace.log`.

# Reference

GitHub issue about the demo test case: https://github.com/etcd-io/etcd/issues/15666

---

# Prerequisites

Install Go as instructed on https://go.dev/doc/install

# Setup GCT
```bash
git clone https://github.com/focs-lab/gct.git
cd gct
go mod tidy
```

# Setup Demo Test Case (etcd)
```bash
git clone https://github.com/etcd-io/etcd.git
cd etcd
git checkout f7af6b64b
```

# Run the test file `etcd/client/v3/txn_test.go` without GCT
```bash
cd client/v3
go test -vet=off -run TestTxnPanics
```
You should see that the test passes. 
Now, try running this test multiple times:

```bash
for i in {1..10}; do go test -vet=off -run TestTxnPanics || break; done;
```

# Run Demo Test Case with GCT
## Step 1: Instrument the target program

Back to the GCT main directory

```bash
cd ../../..
go run ./cmd/gct instrument etcd/client/v3
```

## Step 2: Run instrumented test

```bash
_CCT_TRACE_LOC=trace.log _CCT_SCHEDULER_NAME=random_walk _CCT_RECORD_FLAG=true go test -vet=off -run TestTxnPanics
```

Or use the CLI from the instrumented package directory:

```bash
gct test -vet=off -run TestTxnPanics
```

## Replay Demo Test Case with GCT
```bash
_CCT_TRACE_LOC=trace.log _CCT_SCHEDULER_NAME=replay _CCT_RECORD_FLAG=true go test -vet=off -run TestTxnPanics
```

Or use the CLI from the instrumented package directory:

```bash
gct replay trace.log -vet=off -run TestTxnPanics
```
