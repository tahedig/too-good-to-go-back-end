FROM golang:1.25-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /bin/api ./cmd/api && CGO_ENABLED=0 go build -o /bin/seed ./cmd/seed

FROM alpine:3.20
RUN adduser -D app
COPY --from=build /bin/api /bin/api
COPY --from=build /bin/seed /bin/seed
USER app
EXPOSE 8080
CMD ["/bin/api"]