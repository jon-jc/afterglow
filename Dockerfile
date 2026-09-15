FROM golang:1.27.0-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /afterglow ./cmd/afterglow
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /migrate ./cmd/migrate
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /afterglow /afterglow
COPY --from=build /migrate /migrate
ENV DEMO_MODE=false ADDR=0.0.0.0:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/afterglow"]
