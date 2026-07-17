FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY server ./server
RUN CGO_ENABLED=0 go build -o /careflow-api ./server
FROM alpine:3.22
COPY --from=build /careflow-api /careflow-api
EXPOSE 8080
ENTRYPOINT ["/careflow-api"]
