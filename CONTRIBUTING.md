# Contributing to Latch

Thank you for your interest in improving Latch! To maintain high security standards and code quality, please follow these guidelines.

## Development Workflow

1. **Fork the repository** and create your branch from `main`.
2. **Write clean Go code** without legacy comments (no `// TODO` or commented-out debugging code in production paths).
3. **Keep the data path zero-allocation:** If you modify proxy pipelines, ensure your code does not trigger heap allocations under high load.

## Testing Standards

Before submitting a Pull Request, you must run the local verification suite.

### Run Local Verification Script
We use a PowerShell automation script to format, lint, run race-detector tests, perform quick fuzzing, and verify cross-compilation:
```powershell
.\test.ps1