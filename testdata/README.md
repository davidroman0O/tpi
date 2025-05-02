# Test Data for TPI

This directory contains configuration files and other resources needed for running tests against real Turing Pi hardware.

## Files

- `config.json`: Contains connection details for real Turing Pi hardware
  - Host: 192.168.1.91
  - Username: root
  - Password: `****`
  - API Version: v1-1

```json
{
  "host": "192.168.1.91",
  "username": "root",
  "password": "******",
  "api_version": "v1-1"
} 
```

## Usage

Tests can load this configuration file to connect to real hardware instead of using mocks.

Example:
```go
func loadTestConfig() (*tpi.Client, error) {
    data, err := os.ReadFile("testdata/config.json")
    if err != nil {
        return nil, err
    }
    
    var config struct {
        Host       string `json:"host"`
        Username   string `json:"username"`
        Password   string `json:"password"`
        ApiVersion string `json:"api_version"`
    }
    
    if err := json.Unmarshal(data, &config); err != nil {
        return nil, err
    }
    
    return tpi.NewClient(
        tpi.WithHost(config.Host),
        tpi.WithCredentials(config.Username, config.Password),
        tpi.WithApiVersion(tpi.ApiVersion(config.ApiVersion)),
    )
}
```
