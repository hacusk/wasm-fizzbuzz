var out: [32]u8 = undefined;
var out_len: usize = 0;

const fizz = [_]u8{ 'F', 'i', 'z', 'z' };
const buzz = [_]u8{ 'B', 'u', 'z', 'z' };

fn push(byte: u8) void {
    out[out_len] = byte;
    out_len += 1;
}

fn pushBytes(bytes: []const u8) void {
    for (bytes) |byte| {
        push(byte);
    }
}

fn pushInt(n: i32) void {
    if (n == 0) {
        push('0');
        return;
    }
    var buf: [16]u8 = undefined;
    var value = n;
    var i: usize = 0;
    while (value > 0) {
        buf[i] = '0' + @as(u8, @intCast(@mod(value, 10)));
        i += 1;
        value = @divTrunc(value, 10);
    }
    while (i > 0) {
        i -= 1;
        push(buf[i]);
    }
}

export fn zig_join(fizz_flag: i32, buzz_flag: i32, n: i32) i32 {
    out_len = 0;
    if (fizz_flag != 0) {
        pushBytes(fizz[0..]);
    }
    if (buzz_flag != 0) {
        pushBytes(buzz[0..]);
    }
    if (out_len == 0) {
        pushInt(n);
    }
    return @intCast(out_len);
}

export fn zig_out_ptr() [*]u8 {
    return &out;
}

export fn zig_out_len() i32 {
    return @intCast(out_len);
}
