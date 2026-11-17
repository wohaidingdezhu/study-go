-- KEYS[1] 是一次限流的键。ARGV[1] 为许可次数，ARGV[2] 为窗口长度（毫秒）。
local limit = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
-- 在修改键前校验参数。窗口最多一天，避免把异常输入传给 PEXPIRE。
if not limit or limit < 1 or limit > 1000000 or limit ~= math.floor(limit) or
   not window or window < 1 or window > 86400000 or window ~= math.floor(window) then
    return redis.error_reply("invalid limit or window")
end

-- INCR 与设置过期时间在同一次脚本中完成，其他 Redis 命令不能在两者之间插入。
local count = redis.call("INCR", KEYS[1])
if count == 1 then
    redis.call("PEXPIRE", KEYS[1], window)
end

local allowed = 0
if count <= limit then
    allowed = 1
end

-- 多余请求仍计入次数，但窗口从第一次请求起计算。
return {allowed, count, redis.call("PTTL", KEYS[1])}
