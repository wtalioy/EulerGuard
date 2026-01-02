# Aegis

Aegis is an AI-native security and observability platform powered by eBPF. It combines high-performance kernel monitoring with Large Language Models (LLMs) to provide intelligent diagnostics, natural language querying, and automated threat analysis for Linux systems.

Unlike traditional monitoring tools that simply aggregate logs, Aegis actively interprets system behavior using local or remote AI models to explain *why* an event occurred and assess its security implications in real-time.

## Key Features

### AI-Native Observability

Aegis treats AI as a core component of the monitoring loop, not an afterthought.

- **Intelligent Diagnosis:** Automatically analyzes process trees and event logs to diagnose system anomalies and potential threats.
- **Natural Language Querying:** Interact with system data using natural language through the "DeepAsk" and chat interfaces.
- **Automated Rule Generation:** Leverages AI to generate and refine detection rules based on observed system behavior.

### High-Performance Monitoring

- **eBPF-Based Tracing:** Uses extended Berkeley Packet Filter (eBPF) for low-overhead, safe kernel-level monitoring of processes, file access, and network events.
- **Real-Time Visualization:** Visualizes process hierarchies and event timelines via a modern Vue 3 frontend.
- **Ring Buffer Architecture:** Efficient event handling with configurable ring buffer sizes to prevent data loss under high load.

## prerequisites

Before building Aegis, ensure your development environment has the following tools installed:

- **Linux Kernel**: Recent version supporting eBPF (BTF support recommended).
- **Go**: Version 1.20+ for backend services.
- **Node.js**: Version 18+ for the frontend application.
- **Clang/LLVM**: For compiling eBPF C code.
- **bpftool**: For generating `vmlinux.h`.
- **Ollama (Optional)**: For running local AI models (e.g., `qwen2.5-coder`).

## Getting Started

### 1. Installation

Clone the repository and build the project using the provided Makefile. This will compile the eBPF programs, build the frontend, and package the web server.

``` bash
git clone https://github.com/wtalioy/aegis.git
cd aegis

# Build eBPF, Frontend, and Backend
make web
```

### 2. Configuration

Aegis is configured via `config.yaml`. You must configure the AI section to enable intelligent features. The default configuration supports local inference via Ollama or remote inference via OpenAI-compatible APIs.

``` yaml
# config.yaml example
bpf_path: ./bpf/main.bpf.o
rules_path: ./rules.yaml

ai:
  # Mode: "ollama" (local) or "openai" (remote)
  mode: "ollama"
  
  ollama:
    endpoint: "http://localhost:11434"
    model: "qwen2.5-coder:1.5b"
```

To assist with setting up a local Ollama instance, you can use the included helper script:

``` bash
./scripts/start_ollama.sh "qwen2.5-coder:1.5b"
```

### 3. Usage

Run the web server with root privileges (required for eBPF attachment).

``` bash
# Run the compiled binary
sudo ./build/aegis-web

# Or use the convenience make target
make run
```

Access the dashboard at `http://localhost:3000`.

## Architecture

Aegis consists of three main components:

1. **eBPF Probe (`bpf/`)**: C programs that attach to kernel hooks (tracepoints, kprobes) to capture system events.
2. **Go Backend (`cmd/`, `pkg/`)**: Loads eBPF programs, processes event streams, manages the AI runtime pipeline, and serves the API.
3. **Frontend (`frontend/`)**: A Vue 3 + TypeScript application that provides the visual interface for monitoring and AI interaction.

## Development

The project includes a `Makefile` to streamline development tasks:

- `make bpf`: Compiles only the eBPF object files.
- `make frontend`: Installs dependencies and builds the frontend assets.
- `make web`: Builds the full application bundle.
- `make run`: Builds and runs the application locally with `sudo`.
- `make clean`: Removes build artifacts.

## Support

- **Issues**: Submit bug reports and feature requests via the GitHub Issues page.
- **Documentation**: Refer to the `docs/` directory.
- **Demo Video**: [Aegis Demo Video](https://drive.google.com/file/d/1H05MX3L9eFznvqDZYGqb4l_-r8s-e5y0/view?usp=drive_link)

## License

See the [LICENSE](LICENSE) file for details.