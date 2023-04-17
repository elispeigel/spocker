# Use official golang image as the base image
FROM golang:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the entire project directory to the container
COPY . .

# Install any necessary dependencies
RUN go get -d -v ./...

# Build the project
RUN GOOS=linux GOARCH=amd64 go build -v -o spocker ./cmd/spocker/main.go

# Run the tests
RUN go test -v ./... '-run=^TestCgroup$'
