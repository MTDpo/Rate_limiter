-- Sliding window log: count requests in (now - window, now]; atomic via single EVAL
-- KEYS[1]: rate limit key
-- ARGV[1]: window size (seconds, float)
-- ARGV[2]: max requests per window
-- ARGV[3]: now (unix seconds, float)
-- ARGV[4]: unique ZSET member (caller-generated)
-- ARGV[5]: key TTL (seconds)

local key = KEYS[1]
local window = tonumber(ARGV[1])
local max_requests = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local member = ARGV[4]
local ttl = tonumber(ARGV[5])

local cutoff = now - window
redis.call('ZREMRANGEBYSCORE', key, '-inf', cutoff)
local count = redis.call('ZCARD', key)
if count < max_requests then
  redis.call('ZADD', key, now, member)
  redis.call('EXPIRE', key, ttl)
  return 1
end
return 0
