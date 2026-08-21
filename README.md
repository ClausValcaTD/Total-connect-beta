# Total Connect

Total Connect project structure providing core gRPC/crypto services, CLI interface, and API protocol definitions.

## Project Structure

- `core/`: Core library containing gRPC and crypto dependencies (`golang.org/x/crypto`, `google.golang.org/grpc`).
- `cli/`: Command line client module using Charm libraries (`bubbletea`, `lipgloss`).
- `api/proto/`: Protocol Buffer definitions (`totalconnect.proto`) defining Vault, File, and Status services.
- `Makefile`: Build, test, and proto code generation commands.

## Prerequisites

- **Go**: 1.25+
- **Protobuf Compiler (`protoc`)**: Ensure `protoc` is installed and in your `PATH`.
- **Go Proto Plugins**:
  ```bash
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
  go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
  ```

## Setup & Usage

### 1. Generate Protobuf Code

Generate Go gRPC code from `api/proto/totalconnect.proto`:

```bash
make proto
```

### 2. Build Modules

Build both `core` and `cli` modules:

```bash
make build
```

Or build all modules individually:

```bash
cd core && go build ./...
cd cli && go build ./...
```

### 3. Run Tests

Execute test suites:

```bash
make test
```

## Services Defined in Proto

- **Vault Service**: `Unlock`, `Lock`, `AddCredential`, `GetSSHKey`
- **File Service**: `List`, `Transfer`, `Delete`, `Sync`
- **Status Service**: `Health`, `GetProgress`
