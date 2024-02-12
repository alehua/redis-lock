if redis.call('get', KEYS[1]) == ARGV[1]
then
    -- 刷新过期时间
    redis.call('expire', KEYS[1], ARGV[2])
else
    return redis.call('set', KEYS[1], ARGV[1], 'NX', 'EX', ARGV[2])
end