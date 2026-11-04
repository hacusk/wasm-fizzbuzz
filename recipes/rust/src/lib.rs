#![no_std]

use core::arch::wasm32::unreachable;

#[panic_handler]
fn panic(_: &core::panic::PanicInfo) -> ! {
    unreachable()
}

#[no_mangle]
pub extern "C" fn div3(n: i32) -> i32 {
    if n % 3 == 0 { 1 } else { 0 }
}
