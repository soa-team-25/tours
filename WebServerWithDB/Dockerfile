FROM golang:alpine
WORKDIR /app
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
EXPOSE 3200
EXPOSE 3300
ENTRYPOINT ["go", "run", "main.go"]