static unsigned char buf[64];

extern "C" {

__attribute__((export_name("cpp_ptr")))
unsigned char *cpp_ptr() {
    return buf;
}

__attribute__((export_name("checksum")))
unsigned checksum(int len) {
    unsigned hash = 2166136261u;
    int i;
    if (len < 0 || len > 64) {
        return 0;
    }
    for (i = 0; i < len; i++) {
        hash ^= buf[i];
        hash *= 16777619u;
    }
    return hash;
}

}
