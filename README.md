# Word of Wisdom — PoW TCP Quote Service

## 1) What this service does

A lightweight TCP service that returns a random quote **only after** the client solves a Proof-of-Work (PoW) challenge.

**Protocol (one line per message, terminated with `\n`):**
1. Server sends a **challenge** (JSON): `salt_b64`, `expires`, `difficulty`, `algo`, `version`.
2. Client brute-forces a `nonce`, computing `sha256(payload)` until the number of leading zero bits ≥ `difficulty`,  
   where `payload = salt + ":" + expires + ":" + lower(nonce)`.
3. Client sends a **solution** (JSON): `{"nonce":"..."}`.
4. Server responds with a **quote** (single line) or an error line.

**Server ENV variables:**
- `LISTEN_ADDR` (default `:8080`)
- `POW_DIFFICULTY` (default `22`)
- `POW_TTL` (default `60s`)
- `SHUTDOWN_WAIT` (default `5s`)
- `LOG_LEVEL` (`debug|info|warn|error`, default `info`)

**Client ENV variables:**
- `SERVER_ADDR` (e.g., `localhost:8080` or `server:8080` in Compose)
- `LOG_LEVEL`

---

## Why `sha256` (leading-zero-bits) over alternatives

### PoW requirements for this service
- Simple implementation and cheap verification on the server
- Portability and no external dependencies
- Smooth difficulty tuning via `difficulty`
- Self-contained challenge (no server-side state to validate)

### Advantages of `SHA-256 + leading-zero-bits`
- **Standard library support:** available in Go’s stdlib, cross-platform
- **Cheap verification:** a single `sha256.Sum256` and counting leading zero bits
- **Tunable difficulty:** `difficulty` controls expected search time for a valid `nonce`
- **Stateless validation:** the challenge carries everything (`salt_b64`, `expires`, `difficulty`); the server only enforces TTL
- **Sufficient cryptographic strength** for anti-abuse/rate-limit scenarios (this is not blockchain consensus)

### Why not the alternatives
- **MD5 / SHA-1:** outdated and weaker; no reason to prefer over SHA-256
- **Memory-hard schemes (scrypt/Argon2):** stronger against GPU/ASICs but significantly heavier and more complex—overkill here
- **CAPTCHAs / ML-CAPTCHAs:** require a frontend and third-party services; this protocol is pure TCP
- **VRFs / complex schemes:** don’t improve the practical outcome for this use case

### Conclusion
`sha256` with a leading-zero-bits target is a minimal, clear, and time-tested PoW: easy to implement, easy to test, and provides exactly the barrier needed for a quote-on-demand TCP service.

### Notes
- **Payload format is a hard contract:** `salt:expires:lower(nonce)`. Any deviation (ordering, case, separators) results in `pow verification failed`.
- For demos, you can reduce difficulty (`POW_DIFFICULTY=20`) and/or increase TTL (e.g., `POW_TTL=5m`).
- Distroless images reduce size and attack surface.

---

## 2) How to run

### With Docker Compose

**Start the server**
```bash
# optional: speed up builds
export COMPOSE_BAKE=true

docker compose down -v
docker compose up -d --build
docker compose logs -f server
```

**Fetch a quote (one-off client run)**
```bash
docker compose run --rm client
```
### Locally (without Docker)

**Server (terminal #1):**
```bash
LISTEN_ADDR=:8080 POW_DIFFICULTY=22 POW_TTL=60s SHUTDOWN_WAIT=5s LOG_LEVEL=info \
go run ./cmd/server
```

**Client (terminal #2):**
```bash
SERVER_ADDR=localhost:8080 LOG_LEVEL=info \
go run ./cmd/client
```

## 3) Testing Commands
**All tests**
```bash
go test ./... -count=1
```
**With race detector**
```bash
go test ./... -race -count=1
```

