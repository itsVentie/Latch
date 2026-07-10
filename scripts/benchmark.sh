#!/bin/bash
#!/bin/bash

echo "Running Micro-benchmarks and Allocation Audit"
go test -run=^$ -bench=BenchmarkProxyPipe -benchmem ./internal/network/tests/...

echo -e "\n Generating CPU and Memory Profiles"
go test -run=^$ -bench=BenchmarkProxyPipe -cpuprofile=cpu.prof -memprofile=mem.prof ./internal/network/tests/...

echo -e "\n Top 5 Memory Allocating Functions"
go tool pprof -top -alloc_space mem.prof | head -n 15

echo -e "\n Top 5 CPU Consuming Functions"
go tool pprof -top cpu.prof | head -n 15