const input = new StaticArray<u8>(32);
const output = new StaticArray<u8>(32);

export function as_in_ptr(): i32 {
  return changetype<i32>(input);
}

export function as_out_ptr(): i32 {
  return changetype<i32>(output);
}

export function as_emit(len: i32, checksum: i32): i32 {
  if (len < 0 || len > 32) {
    return -1;
  }
  let hash: u32 = 2166136261;
  for (let i: i32 = 0; i < len; i++) {
    hash = (hash ^ input[i]) * 16777619;
  }
  if (hash != u32(checksum)) {
    return -1;
  }
  for (let i: i32 = 0; i < len; i++) {
    output[i] = input[i];
  }
  return len;
}
