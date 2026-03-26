-- Token Bucket rate limiter - atomic Redis implementation
-- KEYS[1]: rate limit key (e.g., "rate_limit:192.168.1.1")
-- ARGV[1]: capacity (max tokens)
-- ARGV[2]: refill rate (tokens per second)
-- ARGV[3]: current timestamp (Unix seconds)
-- ARGV[4]: tokens to consume (default 1)
-- ARGV[5]: key TTL in seconds

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local refill_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4]) or 1
local ttl = tonumber(ARGV[5]) or 120

local data = redis.call('HMGET', key, 'tokens', 'last_update')
local tokens = tonumber(data[1]) 
local last_update = tonumber(data[2])

if tokens == nil then
    tokens = capacity
    last_update = now
end

-- Refill tokens based on elapsed time
local elapsed = now - last_update
local refill = elapsed * refill_rate
tokens = math.min(capacity, tokens + refill)

if tokens >= requested then
    tokens = tokens - requested
    redis.call('HMSET', key, 'tokens', tokens, 'last_update', now)
    redis.call('EXPIRE', key, ttl)
    return 1 -- allowed
else
    return 0 -- denied
end