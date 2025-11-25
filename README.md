**1. Install dependencies**
```bash
cd /Users/anshu20.kumar/Desktop/assignment
go mod download
```

**2. Run the server**
```bash
# Option 1: Run directly
go run cmd/server/main.go

# Option 2: Build then run
go build -o task-queue cmd/server/main.go
./task-queue
```

**3. Open the dashboard**
```
http://localhost:8080
```

You should see:
```
[INIT] Database initialized
[INIT] Started 3 workers
[INIT] Server starting on http://localhost:8080
```

## ⚖️ Design Trade-offs

When building this, I had to make some technology choices. Here's what I picked and why:

### SQLite instead of Redis/PostgreSQL
**Why SQLite?**
- Zero setup - just a file, no server to manage
- ACID transactions built-in (data safety)
- Perfect for single-machine deployments
- SQL is more flexible than Redis for complex queries

**Trade-off:**
- Can't scale horizontally (stuck on one machine)
- Slower for very high concurrent writes

**When to switch:**
- Use **PostgreSQL** when you need multiple servers or hit 10k+ jobs
- Use **Redis** if you need sub-millisecond reads or a distributed cache

### WebSockets instead of HTTP Polling
**Why WebSockets?**
- Real-time updates with zero delay
- Server can push updates immediately
- Lower bandwidth (one connection vs repeated requests)
- Better user experience

**Trade-off:**
- Requires persistent connection (more server memory)
- Can be blocked by some proxies/firewalls

**Reality:**
- I included polling as a fallback, so if WebSocket fails, it automatically switches to polling every 5 seconds
- Best of both worlds!

### Polling Workers instead of Redis Pub/Sub
**Why polling every 2 seconds?**
- Simple and reliable
- Works with any database (no extra dependencies)
- Easy to debug and understand
- Jobs never get lost

**Trade-off:**
- Jobs can wait up to 2 seconds before processing starts
- Constant database queries (but SQLite handles this easily)

**When to switch:**
- Use **RabbitMQ** or **Redis Streams** if you need sub-second job pickup
- Use push-based systems for high-priority, time-sensitive jobs

### In-Memory Rate Limiter instead of Redis
**Why in-memory?**
- Blazing fast (no network calls)
- Simple code, no dependencies
- Good enough for single-server apps

**Trade-off:**
- Rate limits reset when server restarts
- Can't share limits across multiple servers

**When to switch:**
- Add **Redis** when running multiple server instances
- Use distributed rate limiting for serious production loads

**Bottom line:** These choices prioritize simplicity and "just working" over premature optimization. The system is fast enough for most use cases and can be upgraded piece by piece when you actually hit limits.


