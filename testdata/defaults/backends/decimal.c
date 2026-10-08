#include <stdint.h>
/* The legacy representation is intentionally checked as its current ABI.
   Canonical decimal layout remains a separately governed migration. */
struct legacy_decimal { int64_t coefficient; uint8_t scale; };
extern struct legacy_decimal Decimal(void);
int main(void) {
    struct legacy_decimal value = Decimal();
    return value.coefficient != 0 || value.scale != 1;
}
