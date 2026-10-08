#include <stdint.h>
#include <stdbool.h>
extern int64_t Integer(void), NamedValue(void), ExplicitValue(void);
extern uint64_t Unsigned(void);
extern bool Boolean(void);
extern uint8_t Character(void);
extern uint32_t Rune(void);
int main(void) {
    return Integer() != 0 || Unsigned() != 0 || Boolean()
        || Character() != 0 || Rune() != 0 || NamedValue() != 0
        || ExplicitValue() != 37;
}
