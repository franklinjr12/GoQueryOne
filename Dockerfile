# Multi-stage Dockerfile to build a Windows .exe for a Go/Fyne app

# 1) Builder with Go toolchain and MinGW for CGO cross-compilation
FROM golang:1.22-bookworm AS builder

ENV CGO_ENABLED=1 \
    GOOS=windows \
    GOARCH=amd64 \
    CC=x86_64-w64-mingw32-gcc \
    CXX=x86_64-w64-mingw32-g++

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
        git \
        make \
        pkg-config \
        mingw-w64 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src

# Cache deps first
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source
COPY . .

# Build the Windows GUI executable (no console window)
# Adjust the output name if desired
RUN go build -ldflags "-H=windowsgui" -o /out/GoQueryOne.exe ./cmd


# 2) Minimal final stage that only contains the build artifact
# You can export it with BuildKit: `docker buildx build --output type=local,dest=out .`
FROM scratch AS artifact
COPY --from=builder /out/GoQueryOne.exe /GoQueryOne.exe


