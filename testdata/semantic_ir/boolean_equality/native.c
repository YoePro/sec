#include <stdbool.h>

// Symbols follow the deterministic source order in valid.sec.
extern bool sec_fn_0(bool, bool); // Equal
extern bool sec_fn_1(bool, bool); // Unequal
extern bool sec_fn_4(void);       // BothEqual
extern bool sec_fn_5(void);       // BothUnequal
extern bool sec_fn_6(void);       // ConstantEqual
extern bool sec_fn_7(void);       // ConstantUnequal
extern bool sec_fn_8(bool, bool); // Nested

int main(void) {
    for (int left = 0; left < 2; ++left) {
        for (int right = 0; right < 2; ++right) {
            if (sec_fn_0(left, right) != (left == right)) return 1;
            if (sec_fn_1(left, right) != (left != right)) return 2;
            if (sec_fn_8(left, right) != (left == right)) return 3;
        }
    }
    if (sec_fn_4()) return 4;
    if (!sec_fn_5()) return 5;
    if (sec_fn_6()) return 6;
    if (sec_fn_7()) return 7;
    return 0;
}
