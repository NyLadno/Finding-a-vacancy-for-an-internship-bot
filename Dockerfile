# --- build stage ---
FROM golang:1.27-alpine AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/internship-tracker .

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=build /out/internship-tracker .

# сюда кладём internships.db, чтобы данные не терялись между перезапусками контейнера
VOLUME ["/app"]

EXPOSE 8080

ENTRYPOINT ["./internship-tracker"]
