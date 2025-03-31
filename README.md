# ecosort-be
Go server side application for waste classification

# Description
EcoSort – An AI-Powered Waste Classification

# Getting Started
## Prerequisites
* [go >= 1.23](https://go.dev/dl/)
* [Docker](https://www.docker.com)
* [libtensor](https://www.tensorflow.org/install/lang_c)

## Installation
1. Set the following environment variables in a `.env` within the project root:
```
MODEL_RELEASE_API_KEY # Where the model is downloaded from 
API_REQ_KEY # API Key for accessing the endpoint
```
2. Run
```
go mod tidy
```
3. Start the server
```
go run cmd/server/main.go
```