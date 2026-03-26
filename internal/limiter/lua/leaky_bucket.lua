-- Leaky bucket (fluid): backlog "level" leaks at leak_rate per second; reject if would exceed capacity
-- KEYS[1]: rate limit key
-- ARGV[1]: capacity (max backlog)
-- ARGV[2]: leak_rate (units per second)
-- ARGV[3]: now (unix seconds, float)
-- ARGV[4]: cost of this request (usually 1)
-- ARGV[5]: key TTL (seconds)

local key = KEYS[1]
local capacity = tonumber(ARGV[1])
local leak_rate = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4]) or 1
local ttl = tonumber(ARGV[5]) or 120

local data = redis.call('HMGET', key, 'level', 'last_update')
local level = tonumber(data[1])
local last_update = tonumber(data[2])

if level == nil then
    level = 0
    last_update = now
end

local elapsed = now - last_update
level = math.max(0, level - elapsed * leak_rate)

if level + requested <= capacity then
    level = level + requested
    redis.call('HMSET', key, 'level', level, 'last_update', now)
    redis.call('EXPIRE', key, ttl)
    return 1
else
    return 0
end
