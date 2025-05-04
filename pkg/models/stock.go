package models

// AlphaVantageResponse represents the response from the AlphaVantage API
type AlphaVantageResponse struct {
	MetaData   MetaData              `json:"Meta Data"`
	TimeSeries map[string]DailyPrice `json:"Time Series (Daily)"`
}

// MetaData represents the metadata in the AlphaVantage API response
type MetaData struct {
	Information   string `json:"1. Information"`
	Symbol        string `json:"2. Symbol"`
	LastRefreshed string `json:"3. Last Refreshed"`
	OutputSize    string `json:"4. Output Size"`
	TimeZone      string `json:"5. Time Zone"`
}

// DailyPrice represents a daily price entry in the AlphaVantage API response
type DailyPrice struct {
	Open   string `json:"1. open"`
	High   string `json:"2. high"`
	Low    string `json:"3. low"`
	Close  string `json:"4. close"`
	Volume string `json:"5. volume"`
}

// StockPrice represents a simplified stock price entry
type StockPrice struct {
	Date  string  `json:"date"`
	Close float64 `json:"close"`
}

// StockData represents processed stock data with prices and average
type StockData struct {
	Symbol  string       `json:"symbol"`
	Prices  []StockPrice `json:"prices"`
	Average float64      `json:"average"`
}
