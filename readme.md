# Go Trader

A sophisticated algorithmic trading platform built in Go, powered by Alpaca Markets API and Claude AI.

## Overview

Go Trader is a full-stack algorithmic trading platform that:

- 🌐 Connects to Alpaca Markets API for real-time market data and paper/live trading
- 🧠 Uses Claude AI to generate intelligent trading signals based on market conditions
- 📊 Implements configurable trading algorithms with risk management capabilities
- 📈 Streams real-time market data through a WebSocket server
- 🖥️ Provides a simple web UI for monitoring trades and portfolio performance

## Architecture

The application consists of several key components:

- **Main Server**: Coordinates all components and exposes a REST API
- **Ticker Server**: Streams real-time market data from Alpaca
- **Claude Integration**: Generates trading signals using AI analysis
- **Trading Algorithm**: Executes trades based on signals with risk management
- **Web UI**: Visualizes market data, positions, and trading activity

## Prerequisites

- Go 1.19 or higher
- [Alpaca Markets](https://alpaca.markets/) account (paper trading account is sufficient)
- [Claude API](https://console.anthropic.com/) key (for AI trading signals)

## Setup

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/go-trader.git
   cd go-trader
   ```

2. Create a `.env` file from the template:
   ```
   cp .env.template .env
   ```

3. For live Alpaca-backed operation, edit the `.env` file with your API keys:
   ```
   PAPER_ALPACA_API_KEY=your_alpaca_api_key_here
   PAPER_ALPACA_SECRET_KEY=your_alpaca_secret_key_here
   CLAUDE_API_KEY=your_claude_api_key_here
   ```

   For local development without external credentials, either set `GO_TRADER_MOCK=true` in `.env` or run with `-mock`.

4. Install dependencies:
   ```
   go mod download
   cd web-ui && npm install
   ```

## Running the Application

Start the application with real paper-trading credentials:

```
go run main.go
```

Or start in safe local mock mode without credentials:

```
go run main.go -mock
```

Run the web UI dev server in another terminal:

```
cd web-ui && npm run dev
```

Or with custom backend options:

```
go run main.go -port 8080 -symbols "AAPL,MSFT,TSLA,GOOG,AMZN"
```

### Command-line Options

- `-port`: HTTP server port (default: 8080)
- `-symbols`: Comma-separated list of ticker symbols to track (default: "AAPL,MSFT,TSLA")
- `-paper`: Use paper trading (default: true)
- `-mock`: Run with deterministic mock data and no Alpaca credentials
- `-alpaca-key`: Alpaca API key (overrides env var)
- `-alpaca-secret`: Alpaca secret key (overrides env var)

## API Endpoints

The application exposes the following REST API endpoints:

- `GET /api/account`: Get account information
- `GET /api/positions`: List open positions
- `GET /api/orders`: List recent orders
- `GET /api/tickers`: Get current tracked symbols
- `POST /api/tickers`: Update tracked symbols
- `GET /api/signals`: Get trading signals (optionally filtered by symbol)
- `GET /api/risk-parameters`: Get current risk parameters
- `POST /api/risk-parameters`: Update risk parameters

## WebSocket API

Real-time market data is available via WebSocket:

- Connect to: `ws://localhost:8081/ws`
- Receive real-time ticker data as JSON objects

## Risk Management

The trading algorithm implements several risk management features:

- Maximum position size per symbol
- Maximum percentage of account allocation
- Stop loss percentage 
- Take profit percentage
- Daily loss limit
- Maximum number of open positions

These parameters can be configured via the API.

## Running in Production

For production deployment, consider:

1. Using a process manager like systemd or supervisor
2. Setting up HTTPS with a reverse proxy (Nginx, Caddy, etc.)
3. Switching from paper trading to live trading by updating the Alpaca API URL
4. Implementing more sophisticated logging and monitoring

## License

[MIT License](LICENSE)