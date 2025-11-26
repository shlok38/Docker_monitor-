FROM golang:1.24-alpine

WORKDIR /app

# Copy all files
COPY . .

# Build the application
RUN go build -o docker-monitor .

# Expose the default API port
EXPOSE 8080

# Run in API mode by default
CMD ["./docker-monitor", "-api", "-port", "8080"]
