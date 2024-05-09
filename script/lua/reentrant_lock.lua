-- 若锁不存在, 则新增锁重入次数=1
if (redis.call('exists', KEYS[1])  == 0) then
    redis.call('hset', KEYS[1], ARGV[1], "count", 1)
    redis.call('expire', KEYS[1], ARGV[2])
    return 1
end;

-- 若锁存在, 且唯一标识符匹配, 重入次数+1
if (redis.call('hexists', KEYS[1], ARGV[1]) == 1) then
    local count = redis.call('hincrby', KEYS[1], "count", 1)
    redis.call('expire', KEYS[1], ARGV[2])
    return count
end

return 0