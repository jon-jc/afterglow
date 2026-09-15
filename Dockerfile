FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /afterglow ./cmd/afterglow
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /afterglow /afterglow
ENV DEMO_MODE=false ADDR=0.0.0.0:8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/afterglow"]
