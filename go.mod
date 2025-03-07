module github.com/rileyseaburg/go-trader

go 1.23.4

require (
	github.com/alpacahq/alpaca-trade-api-go/v3 v3.8.1
	github.com/joho/godotenv v1.5.1
	github.com/rileyseaburg/go-trader/algorithm v0.0.0-00010101000000-000000000000
	github.com/rileyseaburg/go-trader/claude v0.0.0-00010101000000-000000000000
	github.com/rileyseaburg/go-trader/ticker v0.0.0-00010101000000-000000000000
	github.com/rileyseaburg/go-trader/types v0.0.0-00010101000000-000000000000
	github.com/shopspring/decimal v1.4.0
)

require (
	cloud.google.com/go v0.118.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/rileyseaburg/go-trader/algorithm/algo v0.0.0-00010101000000-000000000000 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
)

replace github.com/rileyseaburg/go-trader/algorithm => ./algorithm

replace github.com/rileyseaburg/go-trader/algorithm/algo => ./algorithm/algo

replace github.com/rileyseaburg/go-trader/claude => ./claude

replace github.com/rileyseaburg/go-trader/ticker => ./ticker

replace github.com/rileyseaburg/go-trader/types => ./types
