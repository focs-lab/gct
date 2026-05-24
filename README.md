<a href="url"><img src="gct-logo.png" align="right" width="200" ></a>
 
 # GCT: Concurrency Testing for Go

The context below shows how to use GCT to find a reported bug on etcd. 

The GCT is still under development...

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

./gct-instrument.sh etcd/client/v3
```

## Step 2: Run instrumented test

```bash
_CCT_TRACE_LOC=trace.log _CCT_SCHEDULER_NAME=random_walk _CCT_RECORD_FLAG=true go test -vet=off -run TestTxnPanics
```

## Replay Demo Test Case with GCT
```bash
_CCT_TRACE_LOC=trace.log _CCT_SCHEDULER_NAME=replay _CCT_RECORD_FLAG=true go test -vet=off -run TestTxnPanics
```
